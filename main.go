package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/siddontang/go/sync2"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/pborman/uuid"
)

var (
	mtx           sync.RWMutex
	members       = flag.String("members", "", "comma seperated list of members")
	port          = flag.Int("port", 4001, "http port")
	master        = flag.String("redis-master", "", "redis master")
	slaves        = flag.String("redis-slaves", "", "comma seperated list of redis slaves")
	redisDB       = flag.Int("redis-db", 0, "redis db number [0-15]")
	redisPassword = flag.String("redis-pass", "", "redis password")
	items         = map[string]string{}
	broadcasts    *memberlist.TransmitLimitedQueue
)

type broadcast struct {
	msg    []byte
	notify chan<- struct{}
}

type delegate struct{}

type update struct {
	Action string // add, del
	Data   map[string]string
}

type Node struct {
	// Redis address, only support tcp now
	Addr string

	// Replication offset
	Offset int64

	conn *redis.Client
}

// A group contains a Redis master and one or more slaves
// It will use role command per second to check master's alive
// and find slaves automatically.
type Group struct {
	Master *Node
	Slaves map[string]*Node

	CheckErrNum sync2.AtomicInt32

	m sync.Mutex
}

func init() {
	flag.Parse()
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (b *broadcast) Message() []byte {
	return b.msg
}

func (b *broadcast) Finished() {
	if b.notify != nil {
		close(b.notify)
	}
}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (d *delegate) NotifyMsg(b []byte) {
	if len(b) == 0 {
		return
	}

	switch b[0] {
	case 'd': // data
		var updates []*update
		if err := json.Unmarshal(b[1:], &updates); err != nil {
			return
		}
		mtx.Lock()
		for _, u := range updates {
			for k, v := range u.Data {
				switch u.Action {
				case "add":
					items[k] = v
				case "del":
					delete(items, k)
				}
			}
		}
		mtx.Unlock()
	}
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return broadcasts.GetBroadcasts(overhead, limit)
}

func (d *delegate) LocalState(join bool) []byte {
	mtx.RLock()
	m := items
	mtx.RUnlock()
	b, _ := json.Marshal(m)
	return b
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	if len(buf) == 0 {
		return
	}
	if !join {
		return
	}
	var m map[string]string
	if err := json.Unmarshal(buf, &m); err != nil {
		return
	}
	mtx.Lock()
	for k, v := range m {
		items[k] = v
	}
	mtx.Unlock()
}

type eventDelegate struct{}

func (ed *eventDelegate) NotifyJoin(node *memberlist.Node) {
	fmt.Println("A node has joined: " + node.String())
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	fmt.Println("A node has left: " + node.String())
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	fmt.Println("A node was updated: " + node.String())
}

func getRedisClient(addr string, db int) *redis.Client {
	redisOptions := redis.Options{
		Addr: addr,
		DB:   db,
	}
	return redis.NewClient(&redisOptions)
}

func start() (*Group, memberlist.Memberlist, error) {
	hostname, _ := os.Hostname()
	c := memberlist.DefaultLocalConfig()
	c.Events = &eventDelegate{}
	c.Delegate = &delegate{}
	c.BindPort = 0
	c.Name = hostname + "-" + uuid.NewUUID().String()
	m, err := memberlist.Create(c)
	if err != nil {
		return nil, *m, err
	}

	if len(*members) > 0 {
		parts := strings.Split(*members, ",")
		_, err := m.Join(parts)
		if err != nil {
			return nil, *m, err
		}
	}

	//open redis connection at the same time
	var g = new(Group)
	if len(*master) > 0 && len(*slaves) > 0 {
		g.Master = &Node{
			Addr:   *master,
			Offset: 0,
			conn:   getRedisClient(*master, *redisDB),
		}
		g.Slaves = make(map[string]*Node)
		for slave, _ := range strings.Split(*slaves, ",") {
			g.Slaves[string(slave)] = &Node{
				Addr:   string(slave),
				Offset: 0,
				conn:   getRedisClient(string(slave), *redisDB),
			}
		}
	}

	broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return m.NumMembers()
		},
		RetransmitMult: 3,
	}
	node := m.LocalNode()
	fmt.Printf("Local member %s:%d\n", node.Addr, node.Port)
	return g, *m, nil
}

func (g *Group) Close() {
	g.m.Lock()
	defer g.m.Unlock()

	g.Master.close()

	for _, slave := range g.Slaves {
		slave.close()
	}
}

func (n *Node) close() {
	if n.conn != nil {
		_ = n.conn.Close()
		n.conn = nil
	}
}

func main() {
	g, m, err := start()
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("redis master addr: %s", g.Master.Addr)
	if cmd := g.Master.conn.Ping(); cmd == nil {
		panic(cmd)
	}

	// Create a channel to listen for exit signals
	stop := make(chan os.Signal, 1)

	// Register the signals we want to be notified, these 3 indicate exit
	// signals, similar to CTRL+C
	signal.Notify(stop,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP)

	<-stop

	// Leave the cluster with a 5 second timeout. If leaving takes more than 5
	// seconds we return.
	if err := m.Leave(time.Second * 5); err != nil {
		panic(err)
	}

}
