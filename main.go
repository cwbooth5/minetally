package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"minetally/api"
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
var Workers = make(map[api.Worker]map[int]int)
var Users []User

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
	Users         []User `json:"users"`
}

type User struct {
	Name        string   `json:"name"`
	WorkerNames []string `json:"workers"`
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

const PollInterval = 10 * time.Second

func main() {
	var err error
	ConfigureLogging(true, os.Stdout)

	// config file should be stored at:
	// ~/.minetally/tally.json
	home, err := os.UserHomeDir()
	if err != nil {
		LogError.Fatal("ensure ~/.minetally/tally.json config file exists")
	}

	tallyConfig, err := RenderConfig(fmt.Sprintf("%s/.minetally/tally.json", home))
	if err != nil {
		LogError.Fatal("error loading local configuration file")
	}
	WalletAddress = tallyConfig.WalletAddress

	Users = tallyConfig.Users
	LogInfo.Printf("Minetally starting...\nmonitoring address %s\n", WalletAddress)
	LogInfo.Printf("Users: %s\n", Users)
	LogInfo.Printf("Poll Interval: %d\n", PollInterval)

	f, _ := api.FetchBalance(WalletAddress)
	LogInfo.Printf("Wallet Balance: %f\n", f.Balance)

	// Poll forever
	for {
		pollForWorkers()
		pollForShares()

		debug_printShares()
		time.Sleep(PollInterval)
	}
}

func pollForWorkers() {
	response, e := api.FetchWorkers(WalletAddress)
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

func pollForShares() {
	for worker, shares := range Workers {
		response, e := api.FetchWorkerShares(WalletAddress, worker)
		if e != nil {
			LogError.Printf("Failed to poll shares for worker %s", worker.ID)
		} else {
			// Record unique shares
			for _, workerShares := range response.Data {
				shares[workerShares.Date] = workerShares.HashRate
			}

			fmt.Printf("Updated shares for workers.\n")
		}
	}
}

func userForWorker(worker api.Worker) (User, error) {
	var foundUser User
	var found bool

	for _, user := range Users {
		for _, usersWorker := range user.WorkerNames {
			if usersWorker == worker.ID {
				foundUser = user
				found = true
				break
			}
		}
	}

	if found {
		return foundUser, nil
	}
	return foundUser, fmt.Errorf("user not found for worker '%s'", worker.ID)

}

func debug_printShares() {

	sharesByUser := make(map[string]int)
	var totalShares int

	for worker, shares := range Workers {
		fmt.Printf("Worker: %s\n", worker.ID)

		owner, err := userForWorker(worker)
		if err != nil {
			LogError.Println(err)
			continue
		}

		for _, share := range shares {
			if err != nil {
				LogDebug.Println("adding shares to user...")
				sharesByUser[owner.Name] += share
			}

			totalShares += share
		}
	}

	fmt.Printf("Total Shares: %d\n", totalShares)
	for user, shares := range sharesByUser {
		percent := (float64(shares) / float64(totalShares)) * 100.0
		fmt.Printf("User: %s Has shares: %d Percent: %f\n", user, shares, percent)
	}
}
