package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	currentBlock = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "current_block",
		Help: "Latest Block known by node.",
	})
	currentCumulativeWeight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "current_cumulative_weight",
		Help: "Latest Cumulative Weight known by node.",
	})
	currentStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "status",
		Help: "Status: PEERING=1, SYNCING=2, READY=3, MINING=4, UNKNOWN=5",
	})
	currentConnectedPeers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "connected_peers",
		Help: "Current connected peers",
	})
	currentSelfConnectedPeers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "self_connected_peers",
		Help: "Current connected self peers",
	})
	currentCandidatePeers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "candidate_peers",
		Help: "Current candidate peers",
	})
	currentConnectedSyncNodes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "connected_sync_nodes",
		Help: "Current connected sync nodes",
	})
)

var (
	status_request = []byte(`{"jsonrpc": "2.0", "id":"documentation", "method": "getnodestate", "params": []}`)
)

var (
	rcp_address = flag.String("rpc-address", "http://127.0.0.1:3032", "The address of RPC server.")
	listen_port = flag.String("port", ":9090", "The address to listen for metrics server.")
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(currentBlock)
	prometheus.MustRegister(currentCumulativeWeight)
	prometheus.MustRegister(currentStatus)
	prometheus.MustRegister(currentConnectedPeers)
	prometheus.MustRegister(currentSelfConnectedPeers)
	prometheus.MustRegister(currentCandidatePeers)
	prometheus.MustRegister(currentConnectedSyncNodes)
}

type peers []string

type NodestateResult struct {
	Status                     string `json:"status"`
	LatestBlockHeight          int    `json:"latest_block_height"`
	LatestCumulativeWeight     int    `json:"latest_cumulative_weight"`
	NumberOfConnectedPeers     int    `json:"number_of_connected_peers"`
	NumberOfCandidatePeers     int    `json:"number_of_candidate_peers"`
	NumberOfConnectedSyncNodes int    `json:"number_of_connected_sync_nodes"`
	ConnectedPeers             peers  `json:"connected_peers"`
}

type RPCNodestateResponse struct {
	Result NodestateResult `json:"result"`
}

func main() {
	flag.Parse()

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Post(*rcp_address, "application/json", bytes.NewBuffer(status_request))
		if err != nil {
			log.Print("Error getting response. ", err)
			return
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Print("Error reading response. ", err)
			return
		}

		var rpc_result RPCNodestateResponse
		err = json.Unmarshal(body, &rpc_result)
		if err != nil {
			log.Print("Unable to unmarshall")
			return
		}

		currentStatus.Set(getStatus(rpc_result.Result.Status))
		currentBlock.Set(float64(rpc_result.Result.LatestBlockHeight))
		currentCumulativeWeight.Set(float64(rpc_result.Result.LatestCumulativeWeight))
		currentConnectedPeers.Set(float64(rpc_result.Result.NumberOfConnectedPeers))
		currentSelfConnectedPeers.Set(getSelfSegments(rpc_result.Result.ConnectedPeers))
		currentCandidatePeers.Set(float64(rpc_result.Result.NumberOfCandidatePeers))
		currentConnectedSyncNodes.Set(float64(rpc_result.Result.NumberOfConnectedSyncNodes))

		next := promhttp.HandlerFor(
			prometheus.DefaultGatherer, promhttp.HandlerOpts{})
		next.ServeHTTP(w, r)
	})
	fmt.Printf("Starting server at port %s\n", *listen_port)
	if err := http.ListenAndServe(*listen_port, nil); err != nil {
		log.Fatal(err)
	}
}

func getSelfSegments(connectPeers peers) float64 {
	pat := regexp.MustCompile(`^172.16.*`)
	count := 0
	for _, peer := range connectPeers {
		if pat.MatchString(peer) {
			count++
		}
	}
	return float64(count)
}

func getStatus(status string) float64 {
	switch status {
	case "Peering":
		return 1
	case "Syncing":
		return 2
	case "Ready":
		return 3
	case "Mining":
		return 4
	default:
		return 5
	}
}
