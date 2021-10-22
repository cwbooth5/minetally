package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"minetally/api"
	"os"
)

func RenderConfig(file string) (Config, error) {
	var parsed Config
	cfgFile, err := os.Open(file)

	if err != nil {
		LogError.Printf("Error loading config: %s", file)
		return parsed, err
	} else {
		LogInfo.Printf("Config loaded from: %s", file)
	}
	defer cfgFile.Close()

	parser := json.NewDecoder(cfgFile)
	err = parser.Decode(&parsed)

	return parsed, err
}

func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		LogError.Fatal("failed to get home directory")
	}

	return home
}

func getConfigDir() string {
	home := getHomeDir()
	return fmt.Sprintf("%s/.minetally/", home)
}

const configFileName = "tally.json"
const jsonDataFileName = "data.json"

func makeHomeDir(configFileDir string) {
	// Make config dir if it doesn't exist
	if _, err := os.Stat(configFileDir); os.IsNotExist(err) {
		err = os.Mkdir(configFileDir, 0755)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func createConfig(configFileDir string, configFileName string) Config {

	fmt.Println("Enter Wallet Address: (0x000000000000000000000000000)")
	var walletAddress string
	_, err := fmt.Scanln(&walletAddress)
	if err != nil {
		panic(err)
	}

	newConfig := Config{
		WalletAddress: walletAddress,
		Users:         []User{},
	}

	configJson, err := json.Marshal(newConfig)
	if err != nil {
		panic(err)
	}

	makeHomeDir(configFileDir)

	configFilePath := configFileDir + configFileName

	writeStringToFile(configJson, configFilePath)

	return newConfig
}

func saveData() {
	workerData := WorkerData{
		Workers: []api.Worker{},
		Shares:  make(map[int]map[int]int),
	}

	for worker, shares := range Workers {
		workerData.Workers = append(workerData.Workers, worker)

		for date, share := range shares {
			if workerData.Shares[worker.UID] == nil {
				workerData.Shares[worker.UID] = make(map[int]int)
			}

			workerData.Shares[worker.UID][date] = share
		}
	}

	workerJsonData, err := json.Marshal(workerData)
	if err != nil {
		panic(err)
	}

	configDir := getConfigDir()
	dataFile := configDir + jsonDataFileName

	writeStringToFile(workerJsonData, dataFile)

	LogInfo.Println("Data saved to " + dataFile)
}

func writeStringToFile(data []byte, filePath string) {
	fh, err := os.Create(filePath)
	if err != nil {
		LogError.Fatal(err)
	}

	_, err = fh.Write(data)
	if err != nil {
		panic(err)
	}
}

func readData() {
	configDir := getConfigDir()
	dataFile := configDir + jsonDataFileName

	_, err := os.Stat(dataFile)
	if err == nil {
		LogInfo.Printf("Data file exists, reading it into memory")

		// Read data from the file
		data, err := ioutil.ReadFile(dataFile)
		if err != nil {
			LogError.Fatal(err)
		}

		var workerData WorkerData

		// Unmarshall it
		err = json.Unmarshal(data, &workerData)
		if err != nil {
			LogError.Fatal(err)
		}

		// Fill our in-memory data structure with it
		for _, worker := range workerData.Workers {
			Workers[worker] = make(map[int]int)
		}

		// Fill the shares for each worker from the stored data
		for workerUid, shares := range workerData.Shares {
			worker, err := findWorkerForUid(workerUid)
			if err != nil {
				panic(err)
			}

			Workers[worker] = make(map[int]int)
			for date, share := range shares {
				Workers[worker][date] = share
			}
		}
	} else {
		LogInfo.Printf("Data file does not exist! %s", dataFile)
	}
}
