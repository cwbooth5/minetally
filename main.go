package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

/*
	The goal of the program is to hit the API to get per-worker share counts in the pool so we know how to pay out.

	Startup can happen at any time and the API gives us share counts from the last 10 mins at the time of the API call.
	We should be able to call once every 10 minutes (at the max) to maintain uninterrupted share data for each worker.
	Each 10 minute block of data can be recorded to disk for safekeeping.

	presentation (after we have data):
	We want to have share counts for all workers for each hour. 1:00:00 - 1:59:59 (for example)

*/

var WalletAddress string // This will be the address everyone's mining for
var Workers = make(map[Worker]map[int]int)

// Workers is in the format of
//[Worker1]
//	[date] [numshare]
//	[date] [numshare]
//	[date] [numshare]
//[Worker2]
//	[date] [numshare]

// Config holds the wallet address so we don't have to check it in here
type Config struct {
	WalletAddress string `json:"wallet_address"`
}

var (
	LogInfo  *log.Logger
	LogError *log.Logger
	LogDebug *log.Logger
)

// ConfigureLogging will set debug logging up with the -d flag when this program is run.
func ConfigureLogging(debug bool, w io.Writer) {
	LogInfo = log.New(w, "INFO: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	LogError = log.New(w, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)
	if debug {
		LogDebug = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)
	} else {
		LogDebug = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)
		LogDebug.SetOutput(ioutil.Discard)
	}
}

func RenderConfig(file string) (Config, error) {
	var parsed Config
	cfgFile, err := os.Open(file)

	if err != nil {
		LogError.Printf("Error loading config: %s", file)
		return parsed, err
	}
	defer cfgFile.Close()

	parser := json.NewDecoder(cfgFile)
	err = parser.Decode(&parsed)

	return parsed, err
}

/*
{
    "status": true,
    "data": [
        {
            "date": 1620271800,
            "shares": 8,
            "hashrate": 26302
        },
        {
            "date": 1620271200,
            "shares": 2,
            "hashrate": 26370
        },
...
*/

type ChartDataPoint struct {
	Date     int `json:"date"`
	Shares   int `json:"shares"`
	Hashrate int `json:"hashrate"`
}

type ChartResponse struct {
	Status bool             `json:"status"`
	Data   []ChartDataPoint `json:"data"`
}

// Get Chart Data on a wallet for a specific worker
// https://api.nanopool.org/v1/eth/hashratechart/:address/:worker
func GetChartData(worker string) {

}

/*
{
    "status": true,
    "data": [
        {
            "uid": 16818403,
            "id": "DESKTOP-AH56HCB",
            "hashrate": 0,
            "lastShare": 1620277013,
            "rating": 20062
        },
        {
            "uid": 20029185,
            "id": "LAPTOP-707IIDV9",
            "hashrate": 0,
            "lastShare": 1620277218,
            "rating": 9236
        }
    ]
}



*/

type Worker struct {
	UID       int    `json:"uid"`
	ID        string `json:"id"`
	Hashrate  int    `json:"hashrate"`
	LastShare int    `json:"lastShare"` // unix timestamp
	Rating    int    `json:"rating"`
}

type WorkerResponse struct {
	Status bool     `json:"status"`
	Data   []Worker `json:"data"`
}

type SharesResponse struct {
	Status bool     `json:"status"`
	Data   []Shares `json:"data"`
}

type Shares struct {
	Date     int `json:"date"`
	HashRate int `json:"shares"`
}

const PollInterval = 10 * time.Second

func main() {
	// logFile := "tally.log"
	// file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	ConfigureLogging(true, os.Stdout)

	LogInfo.Println("hallo")
	tallyConfig, e := RenderConfig("tally.json")
	if e != nil {
		LogError.Fatal("error loading local configuration file")
	}
	WalletAddress = tallyConfig.WalletAddress
	LogInfo.Printf("Wallet address being monitored: %s\n", WalletAddress)

	// Poll for ever
	for true {
		pollForWorkers()
		pollForShares()

		debug_printShares()
		time.Sleep(PollInterval)
	}
}

func pollForWorkers() {
	response, e := fetchWorkers()
	if e != nil {
		LogError.Println("Failed to poll nanopool")
	} else {
		// we can track workers by their ID here
		numWorkers := len(response.Data)
		fmt.Printf("Found %d workers\n", numWorkers)

		for _, worker := range response.Data {
			if Workers[worker] == nil {
				Workers[worker] = make(map[int]int)
				fmt.Printf("Found new worker! %s\n", worker.ID)
			}
		}
	}
}

func fetchWorkers() (WorkerResponse, error) {
	res, err := http.Get(fmt.Sprintf("https://api.nanopool.org/v1/eth/workers/%s", WalletAddress))
	if err != nil {
		panic(err.Error())
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err.Error())
	}

	var encoded = new(WorkerResponse)
	err = json.Unmarshal(body, &encoded)

	return *encoded, err
}

func pollForShares() {
	for worker, shares := range Workers {
		response, e := fetchWorkerShares(worker)
		if e != nil {
			LogError.Printf("Failed to poll shares for worker %s", worker.ID)
		} else {
			// we can track workers by their ID here
			for _, workerShares := range response.Data {
				shares[workerShares.Date] = workerShares.HashRate
			}

			fmt.Printf("Updated shares for workers.\n")
		}
	}
}

func fetchWorkerShares(worker Worker) (SharesResponse, error) {
	res, err := http.Get(fmt.Sprintf("https://api.nanopool.org/v1/eth/shareratehistory/%s/%s", WalletAddress, worker.ID))
	if err != nil {
		panic(err.Error())
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err.Error())
	}

	var encoded = new(SharesResponse)
	err = json.Unmarshal(body, &encoded)

	return *encoded, err
}

func debug_printShares() {
	for worker, shares := range Workers {
		fmt.Printf("Worker: %s\n", worker.ID)
		debugJson, err := json.Marshal(shares)
		if err != nil {
			panic(err)
		}
		fmt.Println(debugJson)
	}
}
