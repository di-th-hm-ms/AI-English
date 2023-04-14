package lib

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/line/line-bot-sdk-go/linebot"
)

// To generate new access token with JWT assertion

type LineBot struct {
	Client      *linebot.Client
	AccessToken string
}

var bot *LineBot

// To generate JWT assertion
var privateKeyJSON = []byte(fmt.Sprintf(`{
	"alg": "%s",
	"d": "%s",
	"dp": "%s",
	"dq": "%s",
	"e": "%s",
	"kty": "%s",
	"n": "%s",
	"p": "%s",
	"q": "%s",
	"qi": "%s"
  }`, os.Getenv("P_ALG"), os.Getenv("P_D"),
	os.Getenv("P_DP"), os.Getenv("P_DQ"),
	os.Getenv("P_E"), os.Getenv("P_KTY"), os.Getenv("P_N"),
	os.Getenv("P_P"), os.Getenv("P_Q"), os.Getenv("P_QI")))

func base64ToBigInt(base64String string) (*big.Int, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(base64String)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(decoded), nil
}

// Generate assertion JWT
func GenerateJwtAssertion() (string, error) {
	// var privateKey *rsa.PrivateKey
	var privateKeyMap map[string]string
	err := json.Unmarshal(privateKeyJSON, &privateKeyMap)
	// log.Println(privateKeyJSON)
	if err != nil {
		log.Println("while ummarshaling")
		return "", err
	}

	n, err := base64ToBigInt(privateKeyMap["n"])
	if err != nil {
		return "", err
	}

	e, err := base64ToBigInt(privateKeyMap["e"])
	if err != nil {
		return "", err
	}

	d, err := base64ToBigInt(privateKeyMap["d"])
	if err != nil {
		return "", err
	}

	p, err := base64ToBigInt(privateKeyMap["p"])
	if err != nil {
		return "", err
	}

	q, err := base64ToBigInt(privateKeyMap["q"])
	if err != nil {
		return "", err
	}

	dp, err := base64ToBigInt(privateKeyMap["dp"])
	if err != nil {
		return "", err
	}

	dq, err := base64ToBigInt(privateKeyMap["dq"])
	if err != nil {
		return "", err
	}

	qi, err := base64ToBigInt(privateKeyMap["qi"])
	if err != nil {
		return "", err
	}

	privateKey := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: n,
			E: int(e.Int64()),
		},
		D:      d,
		Primes: []*big.Int{p, q},
		Precomputed: rsa.PrecomputedValues{
			Dp:   dp,
			Dq:   dq,
			Qinv: qi,
		},
	}

	// pId := 1660802481
	id := os.Getenv("CHANNEL_ID")
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":       id,
		"sub":       id,
		"aud":       "https://api.line.me/",
		"exp":       time.Now().Add(30 * time.Minute).Unix(), //The expiration time of the JWT.
		"token_exp": 60 * 60 * 24 * 30,                       // This represents a valid expiration time for the channel access token in seconds.
	})

	token.Header["kid"] = os.Getenv("PH_KID")

	jwtString, err := token.SignedString(privateKey)
	if err != nil {
		log.Println("while signing")
		return "", err
	}

	return jwtString, nil
}

// Generate a new access token and register it to LINE server
func GetNewAccessToken() (string, error) {

	jwtAssertion, err := GenerateJwtAssertion()
	if err != nil {
		log.Println("error while generating a JWT: " + err.Error())
		return "", err
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", jwtAssertion)

	req, err := http.NewRequest("POST", "https://api.line.me/oauth2/v2.1/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", errors.New("failed to obtain a new access token")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", errors.New("access token not found in the response")
	}

	return accessToken, nil

}

// check if the accdess token is valid
func VerifyAccessToken(accessToken string) (map[string]interface{}, error) {
	resp, err := http.Get("https://api.line.me/oauth2/v2.1/verify?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// revoke the access token

// Update the access token not in .env but in struct
func NewLineBotClient() *LineBot {
	secret := os.Getenv("CHANNEL_SECRET")

	accessToken, err := GetNewAccessToken()
	if err != nil {
		log.Println("error while generating a new access token: " + err.Error())
	}

	bot, err := linebot.New(secret, accessToken)
	if err != nil {
		// gets this server down temporarily
		log.Fatalf("Failed to create LINE bot client: %v", err)
	}

	return &LineBot{
		Client:      bot,
		AccessToken: accessToken,
	}
}

// refresh an access token periodically
func RefreshTokenPeriodically(bot *LineBot, interval time.Duration) {
	for {
		time.Sleep(interval)
		bot = NewLineBotClient()
		log.Println(bot.AccessToken)
	}
}

// for debug
func InitializeLinebotDebug() {
	lbot, err := linebot.New(os.Getenv("CHANNEL_SECRET"), "KD0gyMj2tlmgrf7sqvlVrsVS4QBKKIeKYfpaV4wQTGFx96iXU9lOUk7eLuzOPY/zwtonHWWhPxPTkpRmz9rqPSDfFJFUTGu7RR4PVn/9ZojYhcvxyWA9RgHAJQXk6Xw3Ad6M8re2P8p1JXMYwU3XrwdB04t89/1O/w1cDnyilFU=")
	if err != nil {
		// gets this server down temporarily
		log.Fatalf("Failed to create LINE bot client: %v", err)
	}

	bot = &LineBot{
		Client:      lbot,
		AccessToken: "KD0gyMj2tlmgrf7sqvlVrsVS4QBKKIeKYfpaV4wQTGFx96iXU9lOUk7eLuzOPY/zwtonHWWhPxPTkpRmz9rqPSDfFJFUTGu7RR4PVn/9ZojYhcvxyWA9RgHAJQXk6Xw3Ad6M8re2P8p1JXMYwU3XrwdB04t89/1O/w1cDnyilFU=",
	}
}

func GetBot() *LineBot {
	return bot
}
