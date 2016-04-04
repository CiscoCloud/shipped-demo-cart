package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"
)

type Item struct {
	Image       string  `json:"image"`
	ItemID      int     `json:"item_id"`
	Name        string  `json:"name"`
	Descrpition string  `json:"description"`
	Price       float64 `json:"price"`
}

type ShoppingCart struct {
	Items []CartItem `json:"items"`
}

type CartItem struct {
	ItemID   int `json:"item_id"`
	Quantity int `json:"quantity"`
}

type Info struct {
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	CardNumber float64 `json:"card_number"`
}

type Response struct {
	Status  string `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const error = "ERROR"
const success = "SUCCESS"

func main() {
	http.HandleFunc("/", HandleIndex)
	http.HandleFunc("/v1/cart/", Cart)
	http.HandleFunc("/v1/order/", Order)

	// The default listening port should be set to something suitable.
	// 8888 was chosen so we could test Catalog by copying into the golang buildpack.
	listenPort := getenv("SHIPPED_CATALOG_LISTEN_PORT", "8888")

	log.Println("Listening on Port: " + listenPort)
	http.ListenAndServe(fmt.Sprintf(":%s", listenPort), nil)
}

// Get environment variable.  Return default if not set.
func getenv(name string, dflt string) (val string) {
	val = os.Getenv(name)
	if val == "" {
		val = dflt
	}
	return val
}

func Cart(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Access-Control-Allow-Origin", "*")

	switch req.Method {
	case "GET":
		// Serve the resource.
		file, e := ioutil.ReadFile("./cart.json")
		if e != nil {
			fmt.Printf("File error: %v\n", e)
			os.Exit(1)
		}
		// Send Cart
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(file))
		log.Println("Sent Cart")
	case "POST":
		url := ""
		// Get item number
		uriSegments := strings.Split(req.URL.Path, "/")

		//  Get item number
		var itemNumber = 0
		if len(uriSegments) >= 3 {
			itemNumber, _ = strconv.Atoi(uriSegments[3])
		}

		// Look for shipped DB
		url = shippedDbCheck(itemNumber)

		mock := mockCheck(req)
		if mock == true {
			url += "?mock=true"
		}
		res, err := http.Get(url)

		if err != nil {
			// Update an existing record.
			rw.WriteHeader(http.StatusInternalServerError)
			errors := response(error, http.StatusInternalServerError, err.Error())
			rw.Write(errors)
			return
		}
		defer res.Body.Close()

		decoder := json.NewDecoder(res.Body)
		var data Item
		err = decoder.Decode(&data)

		if err == nil {
			if data.ItemID != 0 {
				log.Println("Item Exist!")
				fmt.Println(data.Descrpition)

				file, e := ioutil.ReadFile("./cart.json")
				if e != nil {
					fmt.Printf("File error: %v\n", e)
					os.Exit(1)
				}

				var cart ShoppingCart
				err := json.Unmarshal(file, &cart)
				fmt.Println(err)

				if len(cart.Items) > 0 {
					// Quantity increased
					for i := 0; i < len(cart.Items); i++ {
						if cart.Items[i].ItemID == itemNumber {
							addItem(cart, itemNumber, cart.Items[i].Quantity+1, i, false)
							rw.WriteHeader(http.StatusAccepted)
							success := response(success, http.StatusAccepted, "Item: "+strconv.Itoa(itemNumber)+" added to cart!")
							rw.Write(success)
							return
						}
					}
					// New Item added to cart
					fmt.Println(itemNumber)
					addItem(cart, itemNumber, 1, len(cart.Items), true)
					rw.WriteHeader(http.StatusAccepted)
					success := response(success, http.StatusAccepted, "Item: "+strconv.Itoa(itemNumber)+" added to cart!")
					rw.Write(success)
					return
				} else {
					log.Println("Cart is empty")
					addItem(cart, itemNumber, 1, 0, true)
					rw.WriteHeader(http.StatusAccepted)
					success := response(success, http.StatusAccepted, "Item: "+strconv.Itoa(itemNumber)+" added to cart!")
					rw.Write(success)
				}
			}
			// Item doesn't exist
			rw.WriteHeader(http.StatusAccepted)
			success := response(success, http.StatusAccepted, strconv.Itoa(itemNumber)+" doesn't exist, try 1-3")
			rw.Write(success)
		} else {
			// ERROR Connecting to database
			// Item doesn't exist
			rw.WriteHeader(http.StatusInternalServerError)
			success := response(error, http.StatusInternalServerError, "Error connecting to "+url)
			rw.Write(success)
		}
	case "PUT":
		// Update an existing record.
		rw.WriteHeader(http.StatusMethodNotAllowed)
		err := response(error, http.StatusMethodNotAllowed, req.Method+" not allowed")
		rw.Write(err)
	case "DELETE":
		// Remove the record.
		url := ""
		// Get item number
		uriSegments := strings.Split(req.URL.Path, "/")

		//  Get item number
		var itemNumber = 0
		if len(uriSegments) >= 3 {
			itemNumber, _ = strconv.Atoi(uriSegments[3])
		}

		// Look for shipped DB
		url = shippedDbCheck(itemNumber)

		mock := mockCheck(req)
		if mock == true {
			url += "?mock=true"
		}
		res, err := http.Get(url)

		if err != nil {
			// Update an existing record.
			rw.WriteHeader(http.StatusInternalServerError)
			errors := response(error, http.StatusInternalServerError, err.Error())
			rw.Write(errors)
			return
		}
		defer res.Body.Close()

		if mock == true {
			file, e := ioutil.ReadFile("./cart.json")
			if e != nil {
				fmt.Printf("File error: %v\n", e)
				os.Exit(1)
			}

			var cart ShoppingCart
			err := json.Unmarshal(file, &cart)
			fmt.Println(err)

			if len(cart.Items) > 0 {
				for i := 0; i < len(cart.Items); i++ {
					if cart.Items[i].ItemID == itemNumber {
						if cart.Items[i].Quantity > 1 {
							// Reduce quantity
							deleteItem(cart, itemNumber, i, false)
							rw.WriteHeader(http.StatusAccepted)
							success := response(success, http.StatusAccepted, "Item: "+strconv.Itoa(itemNumber)+" has quantity of "+strconv.Itoa(cart.Items[i].Quantity))
							rw.Write(success)
							return
						}
						// Remove Item
						log.Println("remove item")
						deleteItem(cart, itemNumber, i, true)

						rw.WriteHeader(http.StatusAccepted)
						success := response(success, http.StatusAccepted, strconv.Itoa(itemNumber)+" is no longer in your shopping cart.")
						rw.Write(success)
						return
					}
				}
				// Item Doesn't exist
				rw.WriteHeader(http.StatusNotFound)
				error := response(success, http.StatusNotFound, strconv.Itoa(itemNumber)+" isn't in cart.")
				rw.Write(error)
			} else {
				log.Println("Cart is empty")
				rw.WriteHeader(http.StatusNotFound)
				error := response(success, http.StatusNotFound, strconv.Itoa(itemNumber)+" isn't in cart.")
				rw.Write(error)
			}
		} else {
			// Delete DB Item Quantity
		}

	default:
		// Give an error message.
		rw.WriteHeader(http.StatusBadRequest)
		err := response(error, http.StatusBadRequest, "Bad request")
		rw.Write(err)
	}
}

func addItem(cart ShoppingCart, itemID int, itemQuantity int, itemIndex int, appendItem bool) {
	cartItem := CartItem{itemID, itemQuantity}
	if appendItem {
		cart.Items = append(cart.Items, cartItem)
	} else {
		cart.Items[itemIndex].Quantity++
	}
	f, _ := os.OpenFile("cart.json", os.O_RDWR, 0660)
	b, _ := json.Marshal(cart)
	d1 := []byte(b)
	f.Write(d1)
}

func deleteItem(cart ShoppingCart, itemID int, itemIndex int, removeItem bool) {
	// Clear File
	os.Remove("cart.json")
	os.Create("cart.json")

	if !removeItem {
		cart.Items[itemIndex].Quantity--
	} else {
		cart.Items = append(cart.Items[:itemIndex], cart.Items[itemIndex+1:]...)
	}

	f, _ := os.OpenFile("cart.json", os.O_RDWR, 0660)
	b, _ := json.Marshal(cart)
	d1 := []byte(b)
	f.Write(d1)
}

func Order(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Access-Control-Allow-Origin", "*")

	switch req.Method {
	case "GET":
		rw.WriteHeader(http.StatusAccepted)
		success := response(success, http.StatusAccepted, "Order Sent!")
		rw.Write(success)
	case "POST":
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}
		log.Println(string(body))

		var t Info
		err = json.Unmarshal(body, &t)
		if err != nil {
			panic(err)
		}

		// Save Order
		mock := mockCheck(req)
		if mock == true {
			// Clear File
			os.Remove("order.json")
			os.Create("order.json")

			b, err := json.Marshal(t)
			if err != nil {
				fmt.Println(err)
				return
			}
			d1 := []byte(b)
			f, e := os.OpenFile("order.json", os.O_RDWR|os.O_APPEND, 0660)
			defer f.Close()
			if e == nil {
				f.Write(d1)
			}
			log.Println("Order Info Saved")

			rw.WriteHeader(http.StatusAccepted)
			success := response(success, http.StatusAccepted, "Order Info Saved!")
			rw.Write(success)
		} else {
			// Save DB Info

		}
	case "PUT":
		// Update an existing record.
		rw.WriteHeader(http.StatusMethodNotAllowed)
		err := response(error, http.StatusMethodNotAllowed, req.Method+" not allowed")
		rw.Write(err)
	case "DELETE":
		// Remove the record.
		rw.WriteHeader(http.StatusMethodNotAllowed)
		err := response(error, http.StatusMethodNotAllowed, req.Method+" not allowed")
		rw.Write(err)
	default:
		// Give an error message.
		rw.WriteHeader(http.StatusBadRequest)
		err := response(error, http.StatusBadRequest, "Bad request")
		rw.Write(err)
	}

}

func response(status string, code int, message string) []byte {
	resp := Response{status, code, message}
	log.Println(resp.Message)
	response, _ := json.MarshalIndent(resp, "", "    ")

	return response
}

func mockCheck(req *http.Request) bool {
	mock := req.URL.Query().Get("mock")
	if len(mock) != 0 {
		if mock == "true" {
			return true
		}
	}
	return false
}

func shippedDbCheck(itemNumber int) string {
	// Get Catalog Item
	url := ""
	endpoint := os.Getenv("CATALOG_HOST")

	// ip := os.Getenv("TWO_GO_CATALOG_PORT_8888_TCP_ADDR")
	// port := os.Getenv("TWO_GO_CATALOG_PORT_8888_TCP_PORT")
	log.Println("Catalog: ", endpoint)

	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		fmt.Println(pair[0])
	}

	if endpoint != "" {
		url = endpoint + "/v1/catalog/" + strconv.Itoa(itemNumber)
	} else {
		url = "http://localhost:8889/v1/catalog/" + strconv.Itoa(itemNumber)
	}
	return url
}

func HandleIndex(rw http.ResponseWriter, req *http.Request) {
	lp := path.Join("templates", "layout.html")
	fp := path.Join("templates", "index.html")

	// Note that the layout file must be the first parameter in ParseFiles
	tmpl, err := template.ParseFiles(lp, fp)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(rw, nil); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	// Give a success message.
	rw.WriteHeader(http.StatusOK)
	success := response(success, http.StatusOK, "Ready for request.")
	rw.Write(success)
}
