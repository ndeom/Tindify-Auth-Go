package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const (
	stateKey    = "spotify_auth_state"
	scope       = "user-read-private user-read-email user-read-playback-state user-modify-playback-state playlist-modify-public"
	redirectURI = "http://localhost:8080/callback"
	possible    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

func main() {

	//Load environment variables
	godotenv.Load()
	port := os.Getenv("PORT")

	//Handles GET request to the login route
	http.HandleFunc("/api/login", handleLogin)

	//Handles GET requests to the callback route from Spotify
	http.HandleFunc("/callback", handleCallback)

	//Handles GET requests to refresh JSON web tokens
	http.HandleFunc("/api/refresh_token", handleRefresh)

	fmt.Printf("Listening on Port %v...\n", port[1:])

	err := http.ListenAndServe(port, nil)

	if err != nil {
		panic(err.Error())
	}

}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		clientID := os.Getenv("CLIENT_ID")
		fmt.Println("entered login route")

		//Generate random string for Spotify API
		var state string = generateRandomString(16)

		//Set the cookie in response to send to Spotify authorization
		expiration := time.Now().Add(365 * 24 * time.Hour)
		cookie := &http.Cookie{
			Name:    stateKey,
			Value:   state,
			Path:    "/", //Added to mimic default behavior of express middleware cookie-parser
			Expires: expiration,
		}

		http.SetCookie(w, cookie)

		//Enable CORS
		enableCORS(&w)

		//Create query string and redirect
		queryString := "response_type=code&client_id=" + clientID + "&scope=" + scope + "&redirect_uri=" + redirectURI + "&state=" + state
		url := "https://accounts.spotify.com/authorize?" + queryString
		http.Redirect(w, r, url, http.StatusFound)

	} else {

		//If not GET request, redirect back to site with forbidden status
		http.Redirect(w, r, "http://localhost:8081", http.StatusForbidden)

	}
}

func handleCallback(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {

		clientID := os.Getenv("CLIENT_ID")
		clientSecret := os.Getenv("CLIENT_SECRET")

		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		cookie, err := r.Cookie(stateKey)
		if err != nil {
			fmt.Println("Unable to retrieve cookie from header")
		}

		storedState := cookie.Value

		if state == "" || state != storedState {

			http.Redirect(w, r, "http://localhost:8081#error=state_mismatch", http.StatusInternalServerError)

		} else {

			//Clear the cookie data by setting MaxAge = -1
			cookie := http.Cookie{
				Name:   stateKey,
				Value:  "",
				MaxAge: -1,
			}
			http.SetCookie(w, &cookie)

			formQuery := []byte("code=" + code + "&redirect_uri=" + redirectURI + "&grant_type=authorization_code")

			req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", bytes.NewBuffer(formQuery))
			//Add authorization header
			req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))
			//Set Content-Type
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Add("Accept", "application/json")
			//Send the request
			client := &http.Client{}
			resp, err := client.Do(req)
			//If there is an error or the response's status code is not 200, redirect with server error
			if err != nil || resp.StatusCode != 200 {

				fmt.Println("Unable to POST or invalid token")
				http.Redirect(w, r, "http://localhost:8081/#error=invalid_token", http.StatusInternalServerError)

			}
			//Close body at end of main function
			defer resp.Body.Close()
			//Read response body into byte array
			body, err := ioutil.ReadAll(resp.Body)
			//Instantiate a map of strings to arbitrary data types to collect unmarshaled JSON values
			var data map[string]interface{}
			//Unmarshal the byte array from the body to JSON
			if err := json.Unmarshal(body, &data); err != nil {
				panic(err)
			}

			//Convert token to strings
			accessToken := data["access_token"].(string)
			refreshToken := data["refresh_token"].(string)
			urlWithTokens := "http://localhost:8081/#access_token=" + accessToken + "&refresh_token=" + refreshToken

			//Redirect back to website with access token and refresh token
			http.Redirect(w, r, urlWithTokens, http.StatusSeeOther)

		}

	} else {

		//If not GET request, redirect back to site with forbidden status
		http.Redirect(w, r, "http://localhost:8081", http.StatusForbidden)

	}
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {

		fmt.Println("entered the refresh route")

		clientID := os.Getenv("CLIENT_ID")
		clientSecret := os.Getenv("CLIENT_SECRET")

		refreshToken := r.URL.Query().Get("refresh_token")

		formQuery := []byte("grant_type=refresh_token&refresh_token=" + refreshToken)
		req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", bytes.NewBuffer(formQuery))
		//Add authorization header
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))
		//Set Content-Type
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Accept", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)

		if err == nil && resp.StatusCode == 200 {

			fmt.Println("new token on its way")

			//Read response body into byte array
			body, _ := ioutil.ReadAll(resp.Body)
			//Instantiate a map of strings to arbitrary data types to collect unmarshaled JSON values
			var data map[string]interface{}
			//Unmarshal the byte array from the body to JSON
			if err := json.Unmarshal(body, &data); err != nil {
				panic(err)
			}
			//Convert to string
			accessToken := data["access_token"].(string)

			w.Write([]byte("{access_token:" + accessToken + "}"))

		}
		defer resp.Body.Close()

	} else {

		//If not GET request, redirect back to site with forbidden status
		http.Redirect(w, r, "http://localhost:8081", http.StatusForbidden)

	}

}

func randomFloat() float64 {
	rand.Seed(time.Now().UnixNano())
	randInt := rand.Intn(100)
	return float64(randInt) / 100

}

func generateRandomString(length int) string {
	var text string
	possibleLength := len(possible)
	for i := 0; i < length; i++ {
		randomIndexFloat := math.Floor(randomFloat() * float64(possibleLength))
		randomIndex := int64(randomIndexFloat)
		randomChar := possible[randomIndex]
		str := string(randomChar)
		text += str
	}
	return text
}

func enableCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}
