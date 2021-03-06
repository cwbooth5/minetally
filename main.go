package main

import (
	"errors"
	"flag"
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
var Users []User

type WorkerData struct {
	Workers []api.WorkerIdentity `json:"workers"`
	Shares  map[int]map[int]int  `json:"shares"`
}

var Data WorkerData

func hasWorker(data WorkerData, workerUid int) bool {
	_, err := getWorker(data, workerUid)
	return err == nil
}

func getWorker(data WorkerData, workerUid int) (api.WorkerIdentity, error) {
	var foundWorker api.WorkerIdentity
	var found = false

	for _, worker := range data.Workers {
		if worker.UID == workerUid {
			foundWorker = api.WorkerIdentity{
				UID: worker.UID,
				ID:  worker.ID,
			}
			found = true
			break
		}
	}

	if found {
		return foundWorker, nil
	} else {
		return api.WorkerIdentity{}, errors.New("worker not found")
	}
}

// WorkerShares is in the format of
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

const PollInterval = 1 * time.Hour

func main() {
	var err error
	ConfigureLogging(true, os.Stdout)

	var paymentsReport bool
	var usersReport bool
	var configDir string

	defaultConfigDir := getConfigDir()

	flag.BoolVar(&paymentsReport, "payments", false, "Generate payment report")
	flag.BoolVar(&usersReport, "users", false, "Generate user report")
	flag.StringVar(&configDir, "path", defaultConfigDir, "Directory to find config and data")
	flag.Parse()

	anyReports := usersReport || paymentsReport

	// Add a trailing path separator if the dumb ass user missed one
	pathSeparator := string(os.PathSeparator)
	if string(configDir[len(configDir)-1]) != pathSeparator {
		configDir += pathSeparator
	}

	// default config file should be stored at:
	// ~/.minetally/tally.json

	configFilePath := configDir + configFileName

	tallyConfig, err := RenderConfig(configFilePath)
	if err != nil {
		LogError.Printf(err.Error())
		LogError.Println("No config file. Writing default")

		tallyConfig = createConfig(configDir, configFileName)
	}
	WalletAddress = tallyConfig.WalletAddress
	Users = tallyConfig.Users

	// Read the existing data file into memory
	Data, _ = readData(configDir)

	// Generate a report and exit
	if anyReports {
		reportHeader("REPORT")
		if usersReport {
			knownUserReport()
			unknownWorkerReport()
		}
		if paymentsReport {
			printPayoutInfo()
		}
		return
	}

	LogInfo.Printf("Minetally starting...\nmonitoring address %s\n", WalletAddress)
	LogInfo.Printf("Users: %s\n", Users)
	LogInfo.Printf("Poll Interval: %d\n", PollInterval)

	f, _ := api.FetchBalance(WalletAddress)
	LogInfo.Printf("Wallet Balance: %f\n", f.Balance)

	// Poll forever
	for {
		pollForWorkers()
		pollForShares()

		saveData(Data, configDir)

		//debug_printShares()
		LogInfo.Printf("Sleep for: %d", PollInterval)
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
		LogInfo.Printf("Found %d workers\n", numWorkers)

		for _, worker := range response.Data {
			if !hasWorker(Data, worker.UID) {
				workerId := api.WorkerIdentity{
					UID: worker.UID,
					ID:  worker.ID,
				}
				Data.Workers = append(Data.Workers, workerId)
				Data.Shares[worker.UID] = make(map[int]int)
				LogInfo.Printf("Found new worker! %s\n", worker.ID)
			}
		}
	}
}

func pollForShares() {
	for _, worker := range Data.Workers {
		response, e := api.FetchWorkerShares(WalletAddress, worker)
		if e != nil {
			LogError.Printf("Failed to poll shares for worker %s", worker.ID)
		} else {
			shares := Data.Shares[worker.UID]
			// Record unique shares
			for _, workerShares := range response.Data {
				shares[workerShares.Date] = workerShares.HashRate
			}

			LogInfo.Printf("Updated shares for worker %s\n", worker.ID)
		}
	}
}

func userForWorker(worker api.WorkerIdentity) (User, error) {
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
	} else {
		return foundUser, fmt.Errorf("user not found for worker '%s'", worker.ID)
	}
}

func workerIdentityForName(workerName string) (api.WorkerIdentity, error) {
	var foundWorker api.WorkerIdentity
	var found bool

	for _, worker := range Data.Workers {
		if workerName == worker.ID {
			foundWorker = worker
			found = true
			break
		}
	}

	if found {
		return foundWorker, nil
	} else {
		return foundWorker, fmt.Errorf("worker not found for name '%s'", workerName)
	}
}

func workerIdentityForUid(uid int) (api.WorkerIdentity, error) {
	var foundWorker api.WorkerIdentity
	var found bool

	for _, worker := range Data.Workers {
		if uid == worker.UID {
			foundWorker = worker
			found = true
			break
		}
	}

	if found {
		return foundWorker, nil
	} else {
		return foundWorker, fmt.Errorf("worker not found for uid '%d'", uid)
	}
}

func userForWorkerId(workerUid int) (User, error) {
	var foundUser User
	var found bool

	workerIdentity, err := workerIdentityForUid(workerUid)

	if err != nil {
		return foundUser, fmt.Errorf("user not found for worker UID '%d'", workerUid)
	}

	for _, user := range Users {
		for _, usersWorker := range user.WorkerNames {
			if usersWorker == workerIdentity.ID {
				foundUser = user
				found = true
				break
			}
		}
	}

	if found {
		return foundUser, nil
	} else {
		return foundUser, fmt.Errorf("user not found for worker '%c'", workerUid)
	}
}

type MiningTraunch struct {
	StartTime    int64         `json:"start"`
	EndTime      int64         `json:"end"`
	Amount       float64       `json:"amount"`
	WorkerShares map[int]int64 `json:"worker_shares"`
	TotalShares  int64         `json:"total_shares"`
}

func printPayoutInfo() {
	var traunchs []MiningTraunch
	var lastTime = int64(0)

	p, e := api.FetchPayments(WalletAddress)
	if e != nil {
		LogError.Printf("Failed to make Payout request: %s", e.Error())
	} else if p.Status == false {
		LogError.Printf("Payout request Status: false")
	} else {
		// Create the payment traunch date ranges
		for _, payment := range p.Data {

			newTruanch := MiningTraunch{
				StartTime:    lastTime,
				EndTime:      payment.Date,
				Amount:       payment.Amount,
				WorkerShares: map[int]int64{},
			}
			traunchs = append(traunchs, newTruanch)

			lastTime = payment.Date

			/*
				LogInfo.Printf("Payment found:")

				t := time.Unix(payment.Date, 0)
				LogInfo.Printf("Date: %s", t.String())
				LogInfo.Printf("Amount: %f", payment.Amount)

				LogInfo.Printf("\n")
			*/
		}

		sharesReport(traunchs)
	}
}

func sharesReport(traunchs []MiningTraunch) {
	reportHeader("User's Ethereum per Payment")

	// Find all the shares per worker per traunch
	for _, traunch := range traunchs {
		startDate := time.Unix(traunch.StartTime, 0)
		endDate := time.Unix(traunch.EndTime, 0)

		for workerUid := range Data.Shares {
			shares := Data.Shares[workerUid]

			var totalShares int64 = 0
			for timestamp := range shares {
				shareDate := time.Unix(int64(timestamp), 0)

				if (shareDate.After(startDate) && shareDate.Before(endDate)) || shareDate == endDate {
					amountOfShares := shares[timestamp]
					totalShares += int64(amountOfShares)
				}
			}
			traunch.WorkerShares[workerUid] = totalShares
		}
	}

	// Now report Per traunch per user
	for _, traunch := range traunchs {
		userShares := map[string]int64{}
		for _, user := range Users {
			userShares[user.Name] = 0
		}

		for workerId := range traunch.WorkerShares {
			workerShares := traunch.WorkerShares[workerId]
			traunch.TotalShares += workerShares

			user, err := userForWorkerId(workerId)
			if err == nil {
				userShares[user.Name] += workerShares
			} else {
				//LogError.Printf("Failed to find user for worker UID: %d", workerId)
			}
		}

		endTime := time.Unix(traunch.EndTime, 0)
		header := fmt.Sprintf("Payout: %s", endTime.String())
		reportSubheader(header)
		LogInfo.Printf("Ethereum: %f", traunch.Amount)
		var anyoneHasShares = false
		// Print the results from this traunch
		for userName := range userShares {
			shares := userShares[userName]
			if traunch.TotalShares > 0 {
				percentOfShares := shares / traunch.TotalShares
				ethMined := traunch.Amount * float64(percentOfShares)
				LogInfo.Printf("    User: %s", userName)
				LogInfo.Printf("      Shares: %d", shares)
				LogInfo.Printf("      Eth: %d", ethMined)
				anyoneHasShares = true
			} else {
				LogInfo.Printf("    No shares recorded for user: %s", userName)
			}
		}

		if !anyoneHasShares {
			equalProportion := traunch.Amount / float64(len(Users))
			LogInfo.Printf("    No recorded shares for anyone. Equal proportion would be: %f Eth", equalProportion)
		}
	}
}

func knownUserReport() {
	reportSubheader("Known Users")
	for _, user := range Users {
		LogInfo.Printf("User: %s", user.Name)
		var totalUserShares = 0
		for _, workerName := range user.WorkerNames {
			LogInfo.Printf("		Worker: %s", workerName)

			worker, err := workerIdentityForName(workerName)
			if err == nil {
				workerShares := sharedPerWorker(worker)
				totalUserShares += workerShares
				LogInfo.Printf("		  shares: %d", workerShares)
			} else {
				LogInfo.Printf("		  shares: none")
			}
		}
		LogInfo.Printf("		----------------")
		LogInfo.Printf("		total shares: %d", totalUserShares)
		LogInfo.Printf("")
	}
}

func unknownWorkerReport() {
	reportSubheader("Unknown Workers")
	var unknownWorkers []api.WorkerIdentity
	for _, worker := range Data.Workers {
		if !isKnownWorker(worker) && !workerInList(worker.ID, unknownWorkers) {
			unknownWorkers = append(unknownWorkers, worker)
		}
	}

	if len(unknownWorkers) > 0 {
		for _, unknownWorker := range unknownWorkers {
			LogInfo.Printf(unknownWorker.ID)
		}
	} else {
		LogInfo.Printf("-- NONE --")
	}
	LogInfo.Printf("\n")
}

func sharedPerWorker(worker api.WorkerIdentity) int {
	var totalShares = 0

	shares := Data.Shares[worker.UID]
	for _, share := range shares {
		totalShares += share
	}

	return totalShares
}

func workerInList(workerId string, list []api.WorkerIdentity) bool {
	for _, b := range list {
		if b.ID == workerId {
			return true
		}
	}
	return false
}

func isKnownWorker(worker api.WorkerIdentity) bool {
	var isKnown = false
	for _, user := range Users {
		for _, knownWorkerId := range user.WorkerNames {
			if worker.ID == knownWorkerId {
				isKnown = true
				break
			}
		}

		if isKnown {
			break
		}
	}
	return isKnown
}

func reportHeader(title string) {
	LogInfo.Printf("\n")
	LogInfo.Printf("========================================")
	LogInfo.Printf("| %s", title)
	LogInfo.Printf("========================================")
}

func reportSubheader(title string) {
	LogInfo.Printf("")
	LogInfo.Printf("==========")
	LogInfo.Printf("%s", title)
	LogInfo.Printf("==========")
}

func debugPrintShares() {
	sharesByUser := make(map[string]int)
	var totalShares int

	for _, worker := range Data.Workers {
		LogInfo.Printf("Worker: %s\n", worker.ID)
		shares := Data.Shares[worker.UID]

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

	LogInfo.Printf("Total Shares: %d\n", totalShares)
	for user, shares := range sharesByUser {
		percent := (float64(shares) / float64(totalShares)) * 100.0
		LogInfo.Printf("User: %s Has shares: %d Percent: %f\n", user, shares, percent)
	}
}
