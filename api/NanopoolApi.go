package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func FetchWorkers(walletAddress string) (WorkerResponse, error) {
	res, err := http.Get(fmt.Sprintf("https://api.nanopool.org/v1/eth/workers/%s", walletAddress))
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

func FetchWorkerShares(walletAddress string, worker Worker) (SharesResponse, error) {
	res, err := http.Get(fmt.Sprintf("https://api.nanopool.org/v1/eth/shareratehistory/%s/%s", walletAddress, worker.ID))
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
