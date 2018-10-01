package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Angelthump struct {
	UpdateTitle bool   `json:"update_title"`
	Username    string `json:"username"`
	Password    string `json:"password"`

	AccessToken       string
	AccessTokenExpire time.Time
}

type AngelthumpAuthBody struct {
	Strategy string `json:"strategy"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type AngelthumpAuthResponse struct {
	AccessToken string `json:"accessToken"`

	Code    int    `json:"code"`
	Message string `json:"message"`
	Name    string `json:"name"`
}

type AngelthumpTitleBody struct {
	Title string `json:"title"`
}

func (at *Angelthump) Login() error {
	url := "https://angelthump.com/authentication"
	body, _ := json.Marshal(&AngelthumpAuthBody{"local-username", at.Username, at.Password})
	log.Println("angelthump: getting access token...")
	response, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	var atr AngelthumpAuthResponse
	err = json.NewDecoder(response.Body).Decode(&atr)
	if err != nil {
		return err
	}
	if atr.Code > 200 || atr.AccessToken == "" {
		return fmt.Errorf("%s: code %d", atr.Message, atr.Code)
	}
	at.AccessToken = atr.AccessToken
	at.AccessTokenExpire = time.Now().AddDate(0, 0, 7)
	log.Println("angelthump: got access token", atr.AccessToken)

	return nil
}

func (at *Angelthump) ChangeTitle(title string) error {
	if !at.UpdateTitle || at.Username == "" || at.Password == "" || at.AccessToken == "" || title == "" {
		return nil
	}
	if time.Now().After(at.AccessTokenExpire) {
		err := at.Login()
		if err != nil {
			return err
		}
		return at.ChangeTitle(title)
	}
	url := "https://angelthump.com/api/title"
	client := &http.Client{}
	body, _ := json.Marshal(&AngelthumpTitleBody{title})
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-type", "application/json")
	req.AddCookie(&http.Cookie{
		Name:  "angelthump-jwt",
		Value: at.AccessToken,
	})

	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("error updating title: %d", response.StatusCode)
	}
	b, _ := ioutil.ReadAll(response.Body)
	log.Printf("%s", b)
	return nil
}
