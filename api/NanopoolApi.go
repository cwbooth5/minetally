package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

func FetchWorkers(walletAddress string) (WorkerResponse, error) {
	var encoded = new(WorkerResponse)
	res, err := http.Get(fmt.Sprintf("https://api.nanopool.org/v1/eth/workers/%s", walletAddress))
	if err != nil {
		return *encoded, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return *encoded, err
	}

	err = json.Unmarshal(body, &encoded)
	return *encoded, err
}

func FetchWorkerShares(walletAddress string, worker Worker) (SharesResponse, error) {
	var encoded = new(SharesResponse)
	res, err := http.Get(fmt.Sprintf("https://api.nanopool.org/v1/eth/shareratehistory/%s/%s", walletAddress, worker.ID))
	if err != nil {
		return *encoded, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return *encoded, err
	}

	err = json.Unmarshal(body, &encoded)
	return *encoded, err
}

// FetchBalance grabs the ETH balance for the wallet address.
func FetchBalance(walletAddress string) (BalanceResponse, error) {
	res, err := http.Get(fmt.Sprintf("https://api.nanopool.org/v1/eth/balance/%s", walletAddress))
	var encoded = new(BalanceResponse)
	if err != nil {
		return *encoded, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return *encoded, err
	}

	err = json.Unmarshal(body, &encoded)
	return *encoded, err
}

func FetchPayments(walletAddress string) (PaymentsResponse, error) {
	res, err := http.Get(fmt.Sprintf("https://api.nanopool.org/v1/eth/payments/%s", walletAddress))

	//debugResponse(res)

	var encoded = new(PaymentsResponse)
	if err != nil {
		return *encoded, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return *encoded, err
	}

	err = json.Unmarshal(body, &encoded)
	return *encoded, err
}

func debugResponse(response *http.Response) {
	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(string(b))
}
