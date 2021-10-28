package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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

func saveData(data WorkerData, configDir string) {
	workerJsonData, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

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

func readData(configDir string) (WorkerData, error) {
	dataFile := configDir + jsonDataFileName

	_, err := os.Stat(dataFile)
	if err == nil {
		LogInfo.Printf("Data file exists, reading it into memory")

		// Read data from the file
		data, err := ioutil.ReadFile(dataFile)
		if err != nil {
			LogError.Fatal(err)
		}

		workerData := WorkerData{}

		// Unmarshall it
		err = json.Unmarshal(data, &workerData)
		if err != nil {
			LogError.Fatal(err)
		}

		return workerData, err
	} else {
		LogInfo.Printf("Data file does not exist! %s", dataFile)
		return WorkerData{}, err
	}
}
