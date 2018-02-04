#Redis Search Cluster Experimental Client

This is a barebones REST API client for a clustered Redis Search. Following dependencies must be present in the system 
in order to successfully interact with the client. Note that this is just a proof of concept rather than a fully 
working stack.
Feel free to create a PR/bug or reach me at my [email](mailto:slavomir.tecl@gmail.com), contributors are welcome.

####Consul
Consul container/service must be present, the client obtains a list of running Redis servers from it. Later on the 
Consul service can be leveraged to store info about indexes and nodes in cluster.

####Redis
One or more RedisSearch enabled Redis instances with Consul agent, Redis container automatically registers itself with 
the Consul service. Container Dockerfile is available at the following [repo](https://github.com/RedisLabs/redisearch-go)


###Firing up the stack
Run ```docker-compose-up``` to fire it up. If everything is set up correctly you should see three containers running, 
the first one is running Consul, the second one runs Redis and the third one hosts this Client

The consul UI runs on port 8500, the client accepts requests on port 5555

###Available commands

####Synchronize servers
```
curl --request POST --url http://localhost:5555/sync
```

The request tells the client to get all servers with name "redis" from the Consul service

####Schema creation
```
curl --request PUT --url http://localhost:5555/schema
```
Recreates (deletes and creates) the schema on each connected Redis service

####Index files
```
curl --request POST \
  --url http://localhost:5555/doc \
  --header 'content-type: application/json' \
  --data '{
	"Id":"a",
  "Score":1,
	"Payload": "test",
	"Properties": {
		"body": "test",
		"title": "test"
	}
}'
```
Indexes a file to a pseudo-random Redis search, try to put in some more docs with a different id.

####Search files
```
curl --request GET --url http://localhost:5555/search
```
Fires up a search against all redis servers in parallel.

How cool is that :)


