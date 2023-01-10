package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

const NumberOfWorkers = 4

type ParallelCurrency struct {
	Result []CurrencyResult
	Lock   *sync.Mutex
}

type CurrencyPair struct {
	Ask      string `json:"ask"`
	Bid      string `json:"bid"`
	Currency string `json:"currency"`
	Pair     string `json:"pair"`
}

type CurrencyResult struct {
	Currency string
	List     []CurrencyPair
}

func returnErrorMessage(w http.ResponseWriter, msg string, code int) {
	type ErrorMsg struct {
		Message string
	}

	errMsg := ErrorMsg{
		Message: msg,
	}

	enc := json.NewEncoder(w)

	w.WriteHeader(code)
	if err := enc.Encode(errMsg); err != nil {
		return
	}
}

func setDefaultHeaders(w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/json")
}

func getCurrencyRates(currency string) ([]CurrencyPair, error) {
	currency = strings.ToUpper(currency)
	var result []CurrencyPair

	url := fmt.Sprintf("https://api.uphold.com/v0/ticker/%s", currency)

	resp, err := http.Get(url)
	if err != nil {
		return result, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, nil
	}

	if err := json.Unmarshal(b, &result); err != nil {
		return result, err
	}

	return result, nil
}

func addCurrenciesToQueue(currencies []string, queue chan string) {
	for _, currency := range currencies {
		queue <- currency
	}

	close(queue)
}

func getCurrencyRateLock(parallelCurrency *ParallelCurrency, currency string) error {
	currencyPair, err := getCurrencyRates(currency)
	if err != nil {
		return err
	}

	parallelCurrency.Lock.Lock()
	defer parallelCurrency.Lock.Unlock()

	currencyResult := CurrencyResult{
		Currency: currency,
		List:     currencyPair,
	}

	parallelCurrency.Result = append(parallelCurrency.Result, currencyResult)
	return nil
}

func getCurrencyRatesWithWorker(queue chan string, parallelCurrency *ParallelCurrency, waitGroup *sync.WaitGroup) error {
	for currency := range queue {
		if err := getCurrencyRateLock(parallelCurrency, currency); err != nil {
			return err
		}
	}

	waitGroup.Done()
	return nil
}

func main() {
	r := mux.NewRouter()
	handlers.AllowedOrigins([]string{"*"})

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handlers.CORS()(r),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		setDefaultHeaders(w)
		fmt.Fprintf(w, "Server up and running.")
	}).Methods("GET", "POST")

	r.HandleFunc("/currency/{currency}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		currency := vars["currency"]

		result, err := getCurrencyRates(currency)
		if err != nil {
			setDefaultHeaders(w)
			returnErrorMessage(w, "Something went wrong", http.StatusInternalServerError)
			return
		}

		setDefaultHeaders(w)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}).Methods("GET")

	r.HandleFunc("/currency", func(w http.ResponseWriter, r *http.Request) {
		liststr := r.URL.Query().Get("list")
		list := strings.Split(liststr, ",")

		if len(list) < 1 {
			setDefaultHeaders(w)
			returnErrorMessage(w, "No currencies passed in query params", http.StatusBadRequest)
			return
		}

		waitGroup := &sync.WaitGroup{}
		waitGroup.Add(NumberOfWorkers)

		queue := make(chan string)
		go addCurrenciesToQueue(list, queue)

		result := ParallelCurrency{
			Result: []CurrencyResult{},
			Lock:   &sync.Mutex{},
		}
		for i := 0; i < NumberOfWorkers; i++ {
			go getCurrencyRatesWithWorker(queue, &result, waitGroup)
		}

		waitGroup.Wait()

		setDefaultHeaders(w)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result.Result)
	}).Methods("GET")

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
