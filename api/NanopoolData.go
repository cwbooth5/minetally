package api

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

type WorkerIdentity struct {
	UID int    `json:"uid"`
	ID  string `json:"id"`
}

type WorkerResponse struct {
	Status bool     `json:"status"`
	Data   []Worker `json:"data"`
}

type SharesResponse struct {
	Status bool     `json:"status"`
	Data   []Shares `json:"data"`
}

type BalanceResponse struct {
	Status  bool    `json:"status"`
	Balance float64 `json:"data"`
}

type Shares struct {
	Date     int `json:"date"`
	HashRate int `json:"shares"`
}

type Payment struct {
	Date      int64   `json:"date"`
	TxHash    string  `json:"txHash"`
	Amount    float64 `json:"amount"`
	Confirmed bool    `json:"confirmed"`
}

type PaymentsResponse struct {
	Status bool      `json:"status"`
	Data   []Payment `json:"data"`
}
