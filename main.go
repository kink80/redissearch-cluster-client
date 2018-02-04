package main

import (
	"net/http"
	"log"
	"encoding/json"
	"io/ioutil"
	"io"
	"hash/fnv"
	"sync"
	"strconv"
	"os"
	"github.com/kink80/redisearch-go/redisearch"
	"github.com/gorilla/mux"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-cleanhttp"
)

type backend struct {
	servers []*redisearch.Client
	consulConfig *api.Config
}

type results struct {
	document []redisearch.Document
	mutex *sync.Mutex
}

func (r *results) appendDocument(doc redisearch.Document) {
	r.mutex.Lock()
	r.document = append(r.document, doc)
	r.mutex.Unlock()
}

var currentConfig = backend{make([]*redisearch.Client, 0), nil}

func wrapHandler(
	handler func(w http.ResponseWriter, r *http.Request),
) func(w http.ResponseWriter, r *http.Request) {

	h := func(w http.ResponseWriter, r *http.Request) {
		if !userIsAuthorized(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}
	return h
}

func userIsAuthorized(r *http.Request) bool {
	return true
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read the request body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var doc redisearch.Document
	if err := json.Unmarshal(body, &doc); err != nil {
		sendErrorMessage(w, "Could not decode the request body as JSON", http.StatusBadRequest)
		return
	}

	var size = len(currentConfig.servers)
	log.Printf("Servers size: " + strconv.Itoa(size))

	if(size <= 0) {
		sendErrorMessage(w, "No servers found", http.StatusBadRequest)
		return
	}
	var partition = hash(doc.Id) % uint32(size)
	doc.Set("partition", partition)

	var c = currentConfig.servers[partition]
	if err := c.Index("index", doc); err != nil {
		sendErrorMessage(w, "Error indexing doc", http.StatusBadRequest)
		return
	}


	sendJSONResponse(w, doc)
}

func appendDoc(res *results, doc redisearch.Document) {
	res.appendDocument(doc)
}

func schemaHandler(w http.ResponseWriter, r *http.Request) {
	sc := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextField("body")).
		AddField(redisearch.NewTextFieldOptions("title", redisearch.TextFieldOptions{Weight: 5.0, Sortable: true})).
		AddField(redisearch.NewNumericField("date"))

	for _, c := range currentConfig.servers {
		c.Drop("index")
		if err := c.CreateIndex("index", sc); err != nil {
			log.Fatal(err)
		}
	}

	sendJSONResponse(w, "ok")
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	var res = results{make([]redisearch.Document,0), &sync.Mutex{}}

	var n sync.WaitGroup
	var qr = redisearch.NewQuery("test")
	for _, c := range currentConfig.servers {
		n.Add(1)
		go func(client *redisearch.Client, query *redisearch.Query) {
			docs, _, _ := client.Search("index", query)

			for _, d := range docs {
				appendDoc(&res, d)
			}

			n.Done()
		}(c, qr)

	}
	n.Wait()

	sendJSONResponse(w, res.document)
}

func syncRedis(w http.ResponseWriter, r *http.Request) {
	currentConfig.servers = make([]*redisearch.Client, 0)

	log.Println("Connecting to Consul")
	client, err := api.NewClient(currentConfig.consulConfig)
	if err != nil {
		panic(err)
	}
	log.Println("Connected to Consul")

	svc, _, consulErr := client.Catalog().Service("redis", "", nil)
	if consulErr != nil {
		panic(consulErr)
	}

	for _, service := range svc {
		conn := service.Address + ":" + strconv.Itoa(service.ServicePort)
		log.Println("Adding: " + conn)
		client := redisearch.NewClient(conn)
		currentConfig.servers = append(currentConfig.servers, client)
	}

	sendJSONResponse(w, "Synced to redis")
}

func sendErrorMessage(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(status)
	io.WriteString(w, msg)
}

func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	body, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to encode a JSON response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	if err != nil {
		log.Printf("Failed to write the response body: %v", err)
		return
	}
}

func makeRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/doc", wrapHandler(userHandler)).Methods("POST")
	r.HandleFunc("/sync", wrapHandler(syncRedis)).Methods("POST")
	r.HandleFunc("/search", wrapHandler(searchHandler)).Methods("GET")
	r.HandleFunc("/schema", wrapHandler(schemaHandler)).Methods("PUT")
	return r
}

func main() {
	log.SetOutput(os.Stdout)

	consulConn := os.Getenv("CONSUL_MASTER") + ":8500";
	config := &api.Config{
		Address:   consulConn,
		Scheme:    "http",
		Transport: cleanhttp.DefaultPooledTransport(),
	}

	currentConfig.consulConfig = config

	log.Println("Preparing routes")

	r := makeRouter()
	http.Handle("/", r)
	http.ListenAndServe(":5555", r)
}