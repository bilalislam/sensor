package main

import (
	"fmt"
	"github.com/hashicorp/memberlist"
	"net/http"
)

var (
	broadcasts *memberlist.TransmitLimitedQueue
)

func main() {
	err := setupCluster()
	if err != nil {
		fmt.Println(err)
	}

	http.HandleFunc("/add", addHandler)
	fmt.Printf("Listening on :%d\n", 8080)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", 8080), nil); err != nil {
		fmt.Println(err)
	}
}

func setupCluster() error {
	config := memberlist.DefaultLocalConfig()
	list, err := memberlist.Create(config)

	if err != nil {
		return err
	}

	// Create an array of nodes we can join. If you're using a loopback
	// environment you'll need to make sure each node is using its own
	// port. This can be set with the configuration's BindPort field.
	nodes := []string{"0.0.0.0:7946"}
	if _, err := list.Join(nodes); err != nil {
		return err
	}

	broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return list.NumMembers()
		},
		RetransmitMult: 3,
	}

	node := list.LocalNode()
	fmt.Printf("Local member %s:%d\n", node.Addr, node.Port)

	return nil
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("test")
}
