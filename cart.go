package main

import (
	"bytes"
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

type CartStruct struct {
	Carts []Carts
}

type Carts struct {
	User  string `json:"User"`
	Items []Items
}

type Items struct {
	ItemID   int `json:"item_id"`
	Quantity int `json:"quantity"`
}

type AuthResponse struct {
	AccessToken string `json:"accessToken"`
	UserName    string `json:"username"`
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

type Username struct {
	UserName string `json:"username"`
}

const error = "ERROR"
const success = "SUCCESS"

var accountURL = os.Getenv("HOST_SHIPPED_DEMO_ACCOUNT")
var catalogURL = os.Getenv("HOST_SHIPPED_DEMO_CATALOG")

func main() {
	// Assign Env
	http.HandleFunc("/", HandleIndex)
	http.HandleFunc("/v1/cart/", Cart)
	http.HandleFunc("/v1/order/", Order)

	// The default listening port should be set to something suitable.
	listenPort := "8888" //getenv("SHIPPED_CART_LISTEN_PORT", "8888")

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
	rw.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")

	fmt.Println(accountURL)

	fmt.Println(req.Method)
	switch req.Method {
	case "GET":
		break
	case "POST":
		body, err := ioutil.ReadAll(req.Body)
		fmt.Println("USER", string(body))

		if err != nil {
			log.Println(err)
			return
		}
		username := readUsername(body)

		if getUserCartID(username) == -1 {
			// Item doesn't exist
			rw.WriteHeader(http.StatusNotAcceptable)
			success := response(error, http.StatusNotAcceptable, "Please Login")
			rw.Write(success)
		}
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
				userCart := getUserCart(username)

				if len(userCart) > 0 {
					// Quantity increased
					for i := 0; i < len(userCart); i++ {
						if userCart[i].ItemID == itemNumber {
							addItem(itemNumber, userCart[i].Quantity+1, username)
							rw.WriteHeader(http.StatusAccepted)
							success := response(success, http.StatusAccepted, "Item: "+strconv.Itoa(itemNumber)+" added to cart!")
							rw.Write(success)
							return
						}
					}
					// New Item added to cart
					fmt.Println(itemNumber)
					addItem(itemNumber, len(userCart), username)
					rw.WriteHeader(http.StatusAccepted)
					success := response(success, http.StatusAccepted, "Item: "+strconv.Itoa(itemNumber)+" added to cart!")
					rw.Write(success)
					return
				} else {
					log.Println("Cart is empty")
					// Check if Item exist
					for i := 0; i < len(userCart); i++ {
						if userCart[i].ItemID == itemNumber {
							addItem(itemNumber, userCart[i].Quantity+1, username)
							rw.WriteHeader(http.StatusAccepted)
							success := response(success, http.StatusAccepted, "Item: "+strconv.Itoa(itemNumber)+" added to cart!")
							rw.Write(success)
							return
						}
					}
					addItem(itemNumber, 1, username)
					rw.WriteHeader(http.StatusAccepted)
					success := response(success, http.StatusAccepted, "Item: "+strconv.Itoa(itemNumber)+" added to cart!")
					rw.Write(success)
				}
			}
			// Send Cart
			username := readUsername(body)
			userCart := getUserCart(username)

			resp, err := json.MarshalIndent(userCart, "", "    ")
			if err != nil {
				log.Printf("Error marshalling returned catalog item %s", err.Error())
				return
			}
			// Send Cart
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(resp))
			log.Println("Sent Cart")
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
	case "OPTIONS":
		break
	case "DELETE":
		// Remove the record.
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Println(err)
			return
		}
		username := readUsername(body)

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

		userCart := getUserCart(username)
		if len(userCart) > 0 {
			for i := 0; i < len(userCart); i++ {
				if userCart[i].ItemID == itemNumber {
					if userCart[i].Quantity > 1 {
						// Reduce quantity
						deleteItem(itemNumber, username)
						rw.WriteHeader(http.StatusAccepted)
						success := response(success, http.StatusAccepted, "Item: "+strconv.Itoa(itemNumber)+" has quantity of "+strconv.Itoa(userCart[i].Quantity))
						rw.Write(success)
						return
					}
					// Remove Item
					log.Println("remove item")
					deleteItem(itemNumber, username)

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

	default:
		// Give an error message.
		rw.WriteHeader(http.StatusBadRequest)
		err := response(error, http.StatusBadRequest, "Bad request")
		rw.Write(err)
	}
}

func addItem(itemID int, itemQuantity int, username string) {
	f, _ := ioutil.ReadFile("cart.json")
	userID := getUserCartID(username)

	var cart CartStruct
	json.Unmarshal(f, &cart)

	fmt.Println(cart.Carts[userID].User)
	fmt.Println(len(cart.Carts[userID].Items), itemID-1)
	if len(cart.Carts[userID].Items) > itemID-1 {
		fmt.Println("Adding")
		flag := false
		for i := 0; i < len(cart.Carts[userID].Items); i++ {
			if cart.Carts[userID].Items[i].ItemID == itemID {
				cart.Carts[userID].Items[i].Quantity++
				flag = true
			}
		}
		if flag != true {
			var item = Items{itemID, 1}
			cart.Carts[userID].Items = append(cart.Carts[userID].Items, item)
		}
	} else {
		var item = Items{itemID, 1}
		cart.Carts[userID].Items = append(cart.Carts[userID].Items, item)
	}

	os.Remove("cart.json")
	os.Create("cart.json")

	b, e := json.Marshal(cart)
	d1 := []byte(b)
	if e == nil {
		file, _ := os.OpenFile("cart.json", os.O_RDWR, 0660)
		file.Write(d1)
	} else {
		fmt.Println(e)
	}
}

func deleteItem(itemID int, username string) {
	f, _ := ioutil.ReadFile("cart.json")
	userID := getUserCartID(username)

	var cart CartStruct
	json.Unmarshal(f, &cart)

	fmt.Println(cart.Carts[userID].User)
	fmt.Println(len(cart.Carts[userID].Items), itemID-1)
	if len(cart.Carts[userID].Items) > itemID-1 {
		fmt.Println("Deleting")

		for i := 0; i < len(cart.Carts[userID].Items); i++ {
			if cart.Carts[userID].Items[i].ItemID == itemID {
				cart.Carts[userID].Items[i].Quantity--
				if cart.Carts[userID].Items[i].Quantity <= 0 {
					cart.Carts[userID].Items = append(cart.Carts[userID].Items[:i], cart.Carts[userID].Items[i+1:]...)
				}
			}
		}
	}

	os.Remove("cart.json")
	os.Create("cart.json")

	b, e := json.Marshal(cart)
	d1 := []byte(b)
	if e == nil {
		file, _ := os.OpenFile("cart.json", os.O_RDWR, 0660)
		file.Write(d1)
	} else {
		fmt.Println(e)
	}
	// // Clear File
	// os.Remove("cart.json")
	// os.Create("cart.json")
	//
	// if !removeItem {
	// 	cart.Items[itemIndex].Quantity--
	// } else {
	// 	cart.Items = append(cart.Items[:itemIndex], cart.Items[itemIndex+1:]...)
	// }
	//
	// f, _ := os.OpenFile("cart.json", os.O_RDWR, 0660)
	// b, _ := json.Marshal(cart)
	// d1 := []byte(b)
	// f.Write(d1)
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
	case "PUT", "DELETE":
		// Update an existing record.
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

	if catalogURL != "" {
		url = catalogURL + "/v1/catalog/" + strconv.Itoa(itemNumber) // + "?mock=true"
	} else {
		url = "http://localhost:38892/v1/catalog/" + strconv.Itoa(itemNumber)
	}
	return url
}

func getUserCartID(username string) int {
	// Get User
	if !verify(username) {
		return -1
	}

	// Serve the resource.
	file, e := ioutil.ReadFile("./cart.json")
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		os.Exit(1)
	}

	var cart CartStruct
	err := json.Unmarshal(file, &cart)
	fmt.Println(err)

	for i := 0; i < len(cart.Carts); i++ {
		if username == cart.Carts[i].User {
			return i
		}
	}
	return -1
}

func getUserCart(username string) []Items {
	// Get User
	file, e := ioutil.ReadFile("./cart.json")
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		os.Exit(1)
	}

	var cart CartStruct
	err := json.Unmarshal(file, &cart)
	fmt.Println(err)
	var userCart []Items

	for i := 0; i < len(cart.Carts); i++ {
		if username == cart.Carts[i].User {
			userCart = cart.Carts[i].Items
		}
	}
	return userCart
}

func verify(username string) bool {
	// Get User
	endpoint := "/v1/session/"
	var jsonStr = []byte(`{"username":"` + username + `"}`)

	// accountURL = "http://staging--shop--shipped-demo-account--68b68b.gce.shipped-cisco.com/"
	req, err := http.NewRequest("POST", accountURL+endpoint, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))
	return true
}

func readUsername(body []byte) string {
	var user Username
	err := json.Unmarshal(body, &user)
	if err != nil {
		log.Println("Reading Username Error: ", err)
		return "error"
	}
	return user.UserName
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
