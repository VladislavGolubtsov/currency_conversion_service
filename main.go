package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type DataDetail struct {
	CurrencyID string `json:"currency_id"`
	MidRate    string `json:"mid_rate"`
}

type LastUpdated struct {
	LastUpdated string `json:"last_updated"`
}

type Data struct {
	DataDetail []DataDetail `json:"data_detail"`
	DataHeader LastUpdated  `json:"data_header"`
}

type Result struct {
	Data Data `json:"data"`
}

type JSONData struct {
	Result Result `json:"result"`
}

var current_time string
var lastUpdated string

var parsedData JSONData

func parser() {
	// Создаем клиент HTTP
	client := &http.Client{}

	if current_time == "" {
		location, err := time.LoadLocation("Asia/Bangkok")
		if err != nil {
			fmt.Println("Не удалось загрузить локацию:", err)
			return
		}

		currentTime := time.Now().In(location)

		// Форматирование времени в формате "гггг-мм-дд"
		current_time = currentTime.Format("2006-01-02")
	}

	// Создаем новый GET-запрос с указанным URL
	req, err := http.NewRequest("GET", fmt.Sprintf("https://apigw1.bot.or.th/bot/public/Stat-ExchangeRate/v2/DAILY_AVG_EXG_RATE/?start_period=%s&end_period=%s", current_time, current_time), nil)
	if err != nil {
		fmt.Println("Failed to create request:", err)
		return
	}

	// Устанавливаем заголовок X-IBM-Client-Id для идентификации клиента
	req.Header.Set("X-IBM-Client-Id", "c2bbe063-d0ff-456c-bc08-fbd5115fb340")

	// Выполняем запрос с помощью клиента
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Request failed:", err)
		return
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Failed to read response body:", err)
		return
	}

	// Распарсиваем JSON-ответ в структуру данных
	err = json.Unmarshal(data, &parsedData)
	if err != nil {
		fmt.Println("Failed to parse JSON:", err)
		return
	}
	lastUpdated = parsedData.Result.Data.DataHeader.LastUpdated

	// Создаем файл для записи данных
	file, err := os.Create("data.json")
	if err != nil {
		fmt.Println("Failed to create file:", err)
		return
	}
	defer file.Close()

	// Создаем JSON-кодировщик для файла с отступами
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")

	// Записываем распарсенные данные в файл
	err = encoder.Encode(parsedData)
	if err != nil {
		fmt.Println("Failed to write JSON to file:", err)
		return
	}

	fmt.Println("JSON structure saved to data.json", current_time)
}

func fetchCurrencyRates() {
	// Читаем содержимое файла "data.json"
	file, err := ioutil.ReadFile("data.json")
	if err != nil {
		fmt.Println("Failed to read data.json:", err)
		return
	}

	// Распарсиваем JSON-данные из файла
	err = json.Unmarshal(file, &parsedData)
	if err != nil {
		fmt.Println("Failed to parse JSON:", err)
		return
	}

	// Выводим информацию о курсе валют
	fmt.Println("Currency rates:")
	for _, detail := range parsedData.Result.Data.DataDetail {
		fmt.Printf("%s = %s\n", detail.CurrencyID, detail.MidRate)
	}

}

func currencyRatesHandler(w http.ResponseWriter, r *http.Request) {
	// Вызываем функцию для получения данных о курсе валют
	fetchCurrencyRates()

	// Формируем строку с информацией о курсе валют
	var ratesInfo string
	for _, data := range parsedData.Result.Data.DataDetail {
		ratesInfo += fmt.Sprintf("%s = %s\n", data.CurrencyID, data.MidRate)
	}

	// Записываем сформированную строку в http.ResponseWriter
	fmt.Fprint(w, ratesInfo)
}

func convertCurrency(fromCurrency string, toCurrency string, amount float64) (string, error) {
	// Загружаем данные из файла
	file, err := os.Open("data.json")
	if err != nil {
		return "", fmt.Errorf("failed to open data file: %v", err)
	}
	defer file.Close()

	// Читаем данные из файла
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read data file: %v", err)
	}

	// Распарсиваем JSON-данные
	var parsedData JSONData
	err = json.Unmarshal(data, &parsedData)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON data: %v", err)
	}

	// Поиск курсов валют
	fromRateStr := ""
	toRateStr := ""

	for _, detail := range parsedData.Result.Data.DataDetail {
		if fromRateStr != "" && toRateStr != "" {
			break
		}
		if detail.CurrencyID == fromCurrency {
			fromRateStr = detail.MidRate
		} else if detail.CurrencyID == toCurrency {
			toRateStr = detail.MidRate
		}
	}

	// Проверка наличия курсов валют
	if fromCurrency == "THB" {
		fromRateStr = "1"
	} else if fromRateStr == "" {
		return "", fmt.Errorf("currency '%s' not found", fromCurrency)
	}

	if toCurrency == "THB" {
		toRateStr = "1"
	} else if toRateStr == "" {
		return "", fmt.Errorf("currency '%s' not found", toCurrency)
	}

	// Конвертация строковых значений курсов валют в float64
	fromRate, err := strconv.ParseFloat(fromRateStr, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse 'fromRate': %v", err)
	}

	toRate, err := strconv.ParseFloat(toRateStr, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse 'toRate': %v", err)
	}
	fromValue := amount * fromRate
	toValue := fromValue / toRate

	// Форматирование результата
	result := fmt.Sprintf("%.2f %s = %.2f %s", amount, fromCurrency, toValue, toCurrency)

	return result, nil
}

func convertHandler(w http.ResponseWriter, r *http.Request) {
	fromCurrency := r.FormValue("fromCurrency")
	toCurrency := r.FormValue("toCurrency")
	amountStr := r.FormValue("amount")

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	result, err := convertCurrency(fromCurrency, toCurrency, amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, result)
}

func main() {
	parser()

	if lastUpdated != current_time {
		current_time = lastUpdated
		parser()
	}
	go func() {
		for {
			time.Sleep(432 * time.Second)
			parser()
			if lastUpdated != current_time {
				current_time = lastUpdated
				parser()
			}
		}
	}()
	//обработчик который выдает список курса валют (актуальный и всех валют которые парсятся с сайта цб)
	http.HandleFunc("/currency-rates", currencyRatesHandler)

	http.HandleFunc("/convert", convertHandler)

	log.Fatal(http.ListenAndServe(":8000", nil))
}
