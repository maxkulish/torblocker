package main

import (
	"bufio"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

const (
	torExitNodes = "https://check.torproject.org/exit-addresses"
	torBulkIPs = "https://check.torproject.org/torbulkexitlist"
)


func main()  {

	go updateNodes()

	http.HandleFunc("/", nginxHandler)
	log.Fatal(http.ListenAndServe("localhost:8000", nil))
}

// nginxHandler reads Tor exit nodes from local DB
// returns list in nginx format
func nginxHandler(w http.ResponseWriter, r *http.Request)  {
	w.Header().Set("Content-Type", "text/plain")

	nodes, err := getNodes()
	if err != nil {
		fmt.Fprintf(w, "# (no minutes GET variable submitted")
	}
	
	for _, ip := range nodes {
		_, _ = fmt.Fprintf(w, "deny %s;\n", ip)
	}
}

func updateNodes() {
	err := update(torExitNodes)
	if err != nil {
		log.Println(err)
	}

	tick := time.Tick(5 * time.Minute)
	for {
		select {
		case <- tick:
			err := update(torBulkIPs)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func update(url string) (err error) {

	var nodes []string

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	client := http.DefaultClient
	client.Timeout = 30 * time.Second

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	regIP := regexp.MustCompile("(\\d+\\.\\d+\\.\\d+\\.\\d+)")

	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		found := regIP.FindStringSubmatch(line)

		if len(found) > 0 {
			nodes = append(nodes, found[1])
		}
	}

	log.Printf("parsed: %d", len(nodes))

	err = saveToDB(nodes)
	if err != nil {
		return err
	}

	return nil
}

func saveToDB(nodes []string) error  {

	db, err := badger.Open(badger.DefaultOptions("./badger"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for _, ip := range nodes {
		err := db.Update(func(txn *badger.Txn) error {
			now := time.Now().Unix()
			ti := strconv.FormatInt(now, 10)
			err := txn.Set([]byte(ip), []byte(ti))
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	log.Printf("saved to DB: %d", len(nodes))

	return nil
}

func getNodes() (nodes []string, err error) {
	db, err := badger.Open(badger.DefaultOptions("./badger"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			ip := item.Key()
			err := item.Value(func(v []byte) error {
				nodes = append(nodes, string(ip))
				//fmt.Printf("ip=%q, time=%s\n", ip, v)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	log.Printf("got from DB: %d", len(nodes))

	if err != nil {
		return nil, err
	}

	return nodes, nil
}