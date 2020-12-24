package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tomkwok/apprtc-go/collider"
	// "reflect"
)

var TURN_SERVER_OVERRIDE string

const STUN_SERVER_FMT = `
{
    "urls": [
      "stun:%s?transport=udp"
    ]
  }
`
const TURN_SERVER_FMT = `
{
    "urls": [
      "turn:%s?transport=udp"
    ],
	"username": "%s",
	"credential": "%s"
  }
`

var TURN_BASE_URL string = ""
var TURN_URL_TEMPLATE string = `"%s/turn?username=%s&key=%s"`
var CEOD_KEY string = ""

var ICE_SERVER_BASE_URL string = "https://"
var ICE_SERVER_URL_TEMPLATE string = `iceconfig?key=%s`
var ICE_SERVER_API_KEY string = "" //os.environ.get('ICE_SERVER_API_KEY')

const (
	RESPONSE_ERROR            = "ERROR"
	RESPONSE_ROOM_FULL        = "FULL"
	RESPONSE_UNKNOWN_ROOM     = "UNKNOWN_ROOM"
	RESPONSE_UNKNOWN_CLIENT   = "UNKNOWN_CLIENT"
	RESPONSE_DUPLICATE_CLIENT = "DUPLICATE_CLIENT"
	RESPONSE_SUCCESS          = "SUCCESS"
	RESPONSE_INVALID_REQUEST  = "INVALID_REQUEST"

	LOOPBACK_CLIENT_ID = "LOOPBACK_CLIENT_ID"
)

type Client struct {
	Id           string
	is_initiator bool
	messages     []string
	messageLen   int
}

func NewClient(clientId string, initiator bool) *Client {
	return &Client{Id: clientId, is_initiator: initiator, messages: make([]string, 10), messageLen: 0}

}
func (c *Client) AddMessage(msg string) {
	if c.messageLen < len(c.messages) {
		c.messages[c.messageLen] = msg
		c.messageLen = c.messageLen + 1
	}
	//c.messages = append(c.messages,msg)
}
func (c *Client) CleanMessage() {
	c.messageLen = 0
}

func (c *Client) GetMessages() []string {
	return c.messages[0:c.messageLen]
}

func (c *Client) SetInitiator(initiator bool) {
	c.is_initiator = initiator
}

/*
 */
type Room struct {
	Id      string
	clients map[string]*Client
}

//
func NewRoom(roomId string) *Room {
	return &Room{Id: roomId, clients: make(map[string]*Client)}
}

//
func (r *Room) AddClient(c *Client) {
	r.clients[c.Id] = c
}

//
func (r *Room) RemoveClient(client_id string) {
	_, ok := r.clients[client_id]
	if ok {
		delete(r.clients, client_id)
	}
}
func (r *Room) HasClient(client_id string) bool {
	_, ok := r.clients[client_id]
	return ok
}
func (r *Room) GetClient(client_id string) (*Client, bool) {
	client, ok := r.clients[client_id]
	if ok {
		return client, true
	}
	return nil, false
}
func (r *Room) GetOtherClient(client_id string) (*Client, bool) {
	for key, client := range r.clients {
		if key != client_id {
			return client, true
		}
	}
	return nil, false
}

func (r *Room) GetOccupancy() int {
	return len(r.clients)
}

func (r *Room) GetStatus() []string {
	var result []string
	var i int = 0
	result = make([]string, len(r.clients))
	for key, _ := range r.clients {
		result[i] = key
		i = i + 1
	}

	return result
	// abc := map[string]int{
	//     "a": 1,
	//     "b": 2,
	//     "c": 3,
	// }

	// keys := reflect.ValueOf(abc).MapKeys()

	// fmt.Println(keys) // [a b c]
}

var RoomList map[string]*Room

func getRequest(r *http.Request, key, def string) string {
	value := r.Form.Get(key)
	if len(value) == 0 {
		return def
	}
	return value
}
func getWssParameters(r *http.Request) (string, string) {
	// wssHostPortPair := r.Form.Get("wshpp")
	// isTLS := isTLS(r)
	// wssTLS := getRequest(r, "wstls", strconv.FormatBool(isTLS))
	// // http://127.0.0.1:8080/?wstls=false&wshpp=192.168.2.97:4443

	// if len(wssHostPortPair) == 0 {
	// 	log.Println("getWssParameters, r.Host:", r.Host)
	// 	wssHostPortPair = r.Host
	// 	if len(wssHost) > 0 {
	// 		wssHostPortPair = wssHost //+ ":" + strconv.Itoa(wssHostPort) // "192.168.2.30:8089"
	// 	}
	// }
	// log.Println("r:",r)
	// if strings.Index(r.Scheme,"http://") == 0 {
	// 	wssTLS = "false"
	// }
	// wssTLS = "false"
	var wssUrl, wssPostUrl string
	// if strings.EqualFold(wssTLS, "false") {
	if !isTLS(r) {
		// wssUrl = "ws://" + r.Host + ":" + strconv.Itoa(wsHostPort) + "/ws"
		// wssPostUrl = "http://" + r.Host + ":" + strconv.Itoa(httpHostPort)
		wssUrl = "ws://" + r.Host + "/ws"
		wssPostUrl = "http://" + r.Host
	} else {
		// wssUrl = "wss://" + r.Host + ":" + strconv.Itoa(wssHostPort) + "/ws"
		// wssPostUrl = "https://" + r.Host + ":" + strconv.Itoa(httpHostPort)
		wssUrl = "wss://" + r.Host + "/ws"
		wssPostUrl = "https://" + r.Host
	}
	return wssUrl, wssPostUrl
}
func roomPageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("roomPageHandler host:", r.Host, "url:", r.URL.RequestURI(), " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
	room_id := strings.Replace(r.URL.Path, "/r/", "", 1)
	room_id = strings.Replace(room_id, "/", "", -1)
	// todo check whether the room is full with two users
	log.Println("room page room_id:", room_id)
	t, err := template.ParseFiles("./html/index_template.html")
	// t, err := template.ParseFiles("./html/params.html")
	if err != nil {
		log.Println(err)
		log.Println("index template render error:", err.Error())
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	room, ok := RoomList[room_id]

	if ok {
		if room.GetOccupancy() >= 2 {
			t, err = template.ParseFiles("./html/full_template.html")
			if err != nil {

				log.Println("full template render error:", err.Error())
				w.WriteHeader(500)
				w.Write([]byte(err.Error()))
				return

			}
			t.Execute(w, nil)
			return

		}
	}

	data := getRoomParameters(r, room_id, "", nil)

	t.Execute(w, data)
	// t.Execute(w, nil)
}
func addClientToRoom(r *http.Request, room_id, client_id string, is_loopback bool) (map[string]interface{}, bool) {
	room, ok := RoomList[room_id]
	if !ok {
		room = NewRoom(room_id)
		RoomList[room_id] = room
	}
	var is_initiator bool
	var messages []string
	error := ""
	occupancy := room.GetOccupancy()

	if occupancy >= 2 {
		error = RESPONSE_ROOM_FULL
	}
	if room.HasClient(client_id) {
		error = RESPONSE_DUPLICATE_CLIENT
	}
	if 0 == occupancy {
		is_initiator = true
		room.AddClient(NewClient(client_id, is_initiator))
		if is_loopback {
			room.AddClient(NewClient(LOOPBACK_CLIENT_ID, false))
		}
		messages = make([]string, 0)

	} else {
		is_initiator = false
		other_client, _ := room.GetOtherClient(client_id)
		messages = other_client.GetMessages()
		room.AddClient(NewClient(client_id, is_initiator))
		other_client.CleanMessage()
		log.Println("addClientToRoom message:", messages)

	}
	var params map[string]interface{}
	params = make(map[string]interface{})

	params["error"] = error
	params["is_initiator"] = is_initiator
	params["messages"] = messages
	params["room_state"] = room.GetStatus()

	return params, (len(error) == 0)
}

func joinPageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("joinPageHandler host:", r.Host, "url:", r.URL.RequestURI(), " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
	log.Println("joinPageHandler host:", r.Host, "TLS:", r.TLS, " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
	room_id := strings.Replace(r.URL.Path, "/join/", "", 1)
	room_id = strings.Replace(room_id, "/", "", -1)

	log.Println("join page room_id:", room_id)
	client_id := fmt.Sprintf("%d", rand.Intn(1000000000))
	is_loopback := (getRequest(r, "debug", "") == "loopback")

	result, ok := addClientToRoom(r, room_id, client_id, is_loopback)

	var resultStr string
	var returnData map[string]interface{}
	var params map[string]interface{}
	if !ok {
		log.Println("Error adding client to room:", result["error"], ", room_state=", result["room_state"])
		resultStr, _ = result["error"].(string)
		returnData = make(map[string]interface{})
		//return
	} else {
		resultStr = "SUCCESS"
		is_initiator := strconv.FormatBool(result["is_initiator"].(bool))
		log.Println("joinPageHandler  is_initiator:", result["is_initiator"], " String:", is_initiator)
		returnData = getRoomParameters(r, room_id, client_id, is_initiator)
		returnData["messages"] = result["messages"]
		//   returnData["is_initiator"] = "true"
	}

	params = make(map[string]interface{})

	params["result"] = resultStr
	params["params"] = returnData

	//todo output returned json data
	enc := json.NewEncoder(w)
	enc.Encode(&params)
}

func removeClientFromRoom(room_id, client_id string) map[string]interface{} {
	log.Println("removeClientFromRoom room:", room_id, " client:", client_id)
	var result map[string]interface{}
	result = make(map[string]interface{})

	room, ok := RoomList[room_id]

	if !ok {
		log.Println("removeClientFromRoom: Unknow room:", room_id)
		result["error"] = RESPONSE_UNKNOWN_ROOM
		result["room_state"] = ""
		return result
	}

	if !room.HasClient(client_id) {
		log.Println("removeClientFromRoom: Unknow client:", client_id)
		result["error"] = RESPONSE_UNKNOWN_CLIENT
		result["room_state"] = room.GetStatus()
		return result
	}
	room.RemoveClient(client_id)
	room.RemoveClient(LOOPBACK_CLIENT_ID)
	if room.GetOccupancy() > 0 {
		client, _ := room.GetOtherClient(client_id)
		client.SetInitiator(true)
	} else {
		delete(RoomList, room_id)
	}

	result["error"] = ""
	result["room_state"] = room.GetStatus()
	return result
}

func leavePageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("leavePageHandler host:", r.Host, "url:", r.URL.RequestURI(), " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
	//room_id := strings.Replace(r.URL.Path,"/leave/","",1)
	var room_id, client_id string
	url := strings.Split(r.URL.Path, "/")
	log.Println("url array:", url)
	if len(url) >= 3 {
		room_id = url[2]
		client_id = url[3]
		// fmt.Sscanf(r.URL.Path, "/leave/%s/", &room_id, &client_id)
		//log.Println("leave room:",room_id," client:",client_id)
		result := removeClientFromRoom(room_id, client_id)
		//result := removeClientFromRoom(strconv.Itoa(room_id), strconv.Itoa(client_id))
		if len(result["error"].(string)) == 0 {
			log.Println("Room:", room_id, " has state ", result["room_state"])
		}
	}

}
func saveMessageFromClient(room_id, client_id, message_json string) map[string]interface{} {
	log.Println("saveMessageFrom room:", room_id, " client:", client_id, " msg:", message_json)
	var result map[string]interface{}
	result = make(map[string]interface{})
	room, ok := RoomList[room_id]

	result["saved"] = false
	if !ok {
		log.Println("saveMessageFromClient: Unknow room:", room_id)
		result["error"] = RESPONSE_UNKNOWN_ROOM
		return result
	}

	client, has := room.GetClient(client_id)
	if !has {
		log.Println("saveMessageFromclient: Unknow client:", client_id)
		result["error"] = RESPONSE_UNKNOWN_CLIENT
		return result
	}
	if room.GetOccupancy() > 1 {
		result["error"] = ""
		return result
	}

	client.AddMessage(message_json)
	result["error"] = ""
	result["saved"] = true
	return result

}
func messagePageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("messagePageHandler host:", r.Host, "url:", r.URL.RequestURI(), " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
	var room_id, client_id string
	url := strings.Split(r.URL.Path, "/")
	log.Println("url array:", url)
	if len(url) >= 3 {
		room_id = url[2]
		client_id = url[3]

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {

		}
		message_json := string(body)
		result := saveMessageFromClient(room_id, client_id, message_json)

		if !result["saved"].(bool) {
			_, wss_post_url := getWssParameters(r)
			resp, err := http.Post(wss_post_url+"/"+room_id+"/"+client_id, "application/x-www-form-urlencoded", strings.NewReader(message_json))
			if err != nil {
				fmt.Println(err)
			}
			if resp.StatusCode != 200 {
				log.Println("Failed to send message to collider:", resp.StatusCode)
			}
		}

		var params map[string]interface{}
		params = make(map[string]interface{})
		params["result"] = RESPONSE_SUCCESS
		enc := json.NewEncoder(w)
		enc.Encode(&params)
	}
}
func paramsPageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("paramsPageHandler host:", r.Host, "url:", r.URL.RequestURI(), " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
	data := getRoomParameters(r, "", "", nil)
	enc := json.NewEncoder(w)
	enc.Encode(&data)
}
func paramsHTMLPageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("paramsHTMLPageHandler host:", r.Host, "url:", r.URL.RequestURI(), " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
	t, _ := template.ParseFiles("./html/params.html")
	t.Execute(w, nil)

}
func aPageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("aPageHandler host:", r.Host, "url:", r.URL.RequestURI(), " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
}

func iceconfigPageHandler(w http.ResponseWriter, r *http.Request) {

	turnServer := ""
	if len(*flagstun) > 0 {
		turnServer += fmt.Sprintf(STUN_SERVER_FMT, *flagstun)
	}

	if len(*flagturn) > 0 {
		if len(turnServer) > 0 {
			turnServer += ","
		}

		if len(*flagTurnSecret) > 0 {
			timestamp := time.Now().Unix() + 60*60*24
			turnUsername := strconv.Itoa(int(timestamp)) + ":" + *flagTurnUser
			expectedMAC := Hmac(*flagTurnSecret, turnUsername)
			turnServer += fmt.Sprintf(TURN_SERVER_FMT, *flagturn, turnUsername, expectedMAC)
		}
	}
	turnServer = `{"iceServers":[` + turnServer + "]}"
	log.Println("turnServer:", turnServer)
	var dat interface{}
	if err := json.Unmarshal([]byte(turnServer), &dat); err != nil {
		log.Println("json.Unmarshal error:", err)
		return
	}
	// params :=
	enc := json.NewEncoder(w)
	enc.Encode(&dat)
}

func Hmac(key, data string) string {
	// https://stackoverflow.com/questions/30745153/turn-server-for-webrtc-with-rest-api-authentication?noredirect=1&lq=1
	hmac := hmac.New(sha1.New, []byte(key))
	hmac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(hmac.Sum(nil))
	// return base64.StdEncoding.EncodeToString(hmac.Sum([]byte("")))
}

func computePageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("computePagehandler host:", r.Host, "url:", r.URL.RequestURI(), " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
}
func mainPageHandler(w http.ResponseWriter, r *http.Request) {
	// if r.URL.Path == "/" {
	//     http.Redirect(w, r, "/login/index", http.StatusFound)
	// }
	log.Println("host:", r.Host, "url:", r.URL.RequestURI(), " path:", r.URL.Path, " raw query:", r.URL.RawQuery)
	t, err := template.ParseFiles("./html/index_template.html")
	// t, err := template.ParseFiles("./html/params.html")
	if err != nil {
		log.Println(err)
	}

	data := getRoomParameters(r, "", "", nil)

	t.Execute(w, data)
	// t.Execute(w, nil)

}

func isTLS(r *http.Request) bool {
	return (r.TLS != nil)
}

func getRoomParameters(r *http.Request, room_id, client_id string, is_initiator interface{}) map[string]interface{} {

	var data map[string]interface{}

	data = make(map[string]interface{})
	data["error_messages"] = []string{}

	data["warning_messages"] = []string{}
	data["is_loopback"] = false                                                                                     // json.dumps(debug == 'loopback'),
	data["pc_config"] = template.JS(`{"iceServers": [], "rtcpMuxPolicy": "require", "bundlePolicy": "max-bundle"}`) //json.dumps(pc_config),
	data["pc_constraints"] = template.JS(`{"optional": []}`)                                                        // json.dumps(pc_constraints),
	data["offer_options"] = template.JS("{}")                                                                       //json.dumps(offer_options),
	data["media_constraints"] = template.JS(`{"video": {"optional": [{"minWidth": "1280"}, {"minHeight": "720"}], "mandatory": {}}, "audio": true}`)

	// var dat []map[string]interface{}
	var dat interface{}
	if err := json.Unmarshal([]byte(TURN_SERVER_OVERRIDE), &dat); err != nil {
		log.Println("json.Unmarshal error:", err)
	}
	// log.Println(dat)
	data["turn_server_override"] = dat // template.JS(strings.Replace(TURN_SERVER_OVERRIDE,"\n","",-1) )

	username := fmt.Sprintf("%d", rand.Intn(1000000000))
	if len(client_id) > 0 {
		username = client_id
	}
	data["turn_url"] = template.JS(fmt.Sprintf(TURN_URL_TEMPLATE, TURN_BASE_URL, username, CEOD_KEY))

	// ice_server_base_url := getRequest(r, "ts", ICE_SERVER_BASE_URL)
	data["ice_server_url"] = template.JS(fmt.Sprintf(ICE_SERVER_URL_TEMPLATE, ICE_SERVER_API_KEY))
	// if isTLS(r) {
	// 	// data["ice_server_url"] = template.JS(fmt.Sprintf(ICE_SERVER_URL_TEMPLATE, "https", r.Host, httpsHostPort, ICE_SERVER_API_KEY))
	// 	data["ice_server_url"] = template.JS(fmt.Sprintf(ICE_SERVER_URL_TEMPLATE, "https", r.Host, ICE_SERVER_API_KEY))
	// } else {
	// 	// data["ice_server_url"] = template.JS(fmt.Sprintf(ICE_SERVER_URL_TEMPLATE, "http", r.Host, httpHostPort, ICE_SERVER_API_KEY))
	// 	data["ice_server_url"] = template.JS(fmt.Sprintf(ICE_SERVER_URL_TEMPLATE, "http", r.Host, ICE_SERVER_API_KEY))
	// }

	data["ice_server_transports"] = getRequest(r, "tt", "")

	var dtls, include_loopback_js string

	debug := getRequest(r, "debug", "")
	if strings.EqualFold(debug, "loopback") {
		dtls = "false"
		include_loopback_js = "<script src=\"/js/loopback.js\"></script>"
	} else {
		dtls = "true"
		include_loopback_js = ""
	}
	data["include_loopback_js"] = include_loopback_js
	data["ddtls"] = dtls

	include_rtstats_js := ""
	data["include_rtstats_js"] = include_rtstats_js

	wss_url, wss_post_url := getWssParameters(r)
	data["wss_url"] = template.URL(wss_url)
	data["wss_post_url"] = template.URL(wss_post_url)

	bypass_join_confirmation := false
	data["bypass_join_confirmation"] = bypass_join_confirmation

	data["version_info"] = template.JS(`{}`)

	if len(room_id) > 0 {
		var room_link string
		if isTLS(r) {
			room_link = "https://" + r.Host + "/r/" + room_id + "?" + r.URL.RawQuery
		} else {
			room_link = "http://" + r.Host + "/r/" + room_id + "?" + r.URL.RawQuery
		}
		// room_link := r.Host + "/r/" + room_id + "?" + r.URL.RawQuery

		// log.Println("host:",r.Host,"  url:",r.URL.String," uri:",r.URL.RequestURI)
		data["room_id"] = room_id
		data["room_link"] = template.URL(room_link)
	}
	if len(client_id) > 0 {
		data["client_id"] = client_id
	}
	if is_initiator != nil {
		data["is_initiator"] = is_initiator
	}

	return data
}

// var useTls bool
var wssHostPort int
var wsHostPort int
var httpsHostPort string
var httpHostPort string

// var wssHost string

// var flagUseTls = flag.Bool("tls", true, "TLS is used or not")
// var flagWssHostPort = flag.Int("wssport", 1443, "The TCP port that the server listens on")
// var flagWsHostPort = flag.Int("wsport", 2443, "The TCP port that the server listens on")
var flagHttpsHostPort = flag.String("https", ":8888", "address:port that http server listens on")
var flagHttpHostPort = flag.String("http", ":8080", "address:port that https server listens on")

// var flagWssHost = flag.String("host", "192.168.2.30", "your hostname or host ip")
var flagstun = flag.String("stun", "", "stun server host:port")
var flagturn = flag.String("turn", "", "turn server host:port")
var flagTurnUser = flag.String("turn-username", "username", "turn server username")
var flagTurnPassword = flag.String("turn-password", "password", "turn server user password")
var flagTurnSecret = flag.String("turn-static-auth-secret", "", "turn server static auth secret")

// var roomSrv = flag.String("room-server", "https://appr.tc", "origin of room server")

var CERT = flag.String("cert", "./fullchain.pem", "https cert pem file")
var KEY = flag.String("key", "./privkey.pem", "https cert key file")

var ice_server_url string

func main() {
	flag.Parse()
	// useTls = *flagUseTls
	// wssHostPort = *flagWssHostPort
	// wsHostPort = *flagWsHostPort
	httpsHostPort = *flagHttpsHostPort
	httpHostPort = *flagHttpHostPort
	// wssHost = *flagWssHost

	if len(*flagturn) > 0 {
		if len(*flagTurnUser) == 0 {
			log.Printf("If turn server is set, turn-username must be set")
			return
		}

		if len(*flagTurnPassword) == 0 && len(*flagTurnSecret) == 0 {
			log.Printf("If turn server is set, turn-password or turn-static-auth-secret must be set")
			return
		}
	}

	if len(*flagstun) > 0 {
		TURN_SERVER_OVERRIDE += fmt.Sprintf(STUN_SERVER_FMT, *flagstun)
	}
	if len(*flagturn) > 0 {
		if len(TURN_SERVER_OVERRIDE) > 0 {
			TURN_SERVER_OVERRIDE += ","
		}

		if len(*flagTurnSecret) > 0 {
			timestamp := time.Now().Unix() + 60*60*24
			turnUsername := strconv.Itoa(int(timestamp)) + ":" + *flagTurnUser
			expectedMAC := Hmac(*flagTurnSecret, turnUsername)
			TURN_SERVER_OVERRIDE += fmt.Sprintf(TURN_SERVER_FMT, *flagturn, turnUsername, expectedMAC)
		}
	}
	TURN_SERVER_OVERRIDE = "[" + TURN_SERVER_OVERRIDE + "]"
	log.Printf("TURN_SERVER_OVERRIDE: %s", TURN_SERVER_OVERRIDE)

	RoomList = make(map[string]*Room)
	WebServeMux := http.NewServeMux()
	WebServeMux.Handle("/css/", http.FileServer(http.Dir("./")))
	WebServeMux.Handle("/js/", http.FileServer(http.Dir("./")))
	WebServeMux.Handle("/images/", http.FileServer(http.Dir("./")))
	WebServeMux.Handle("/favicon.ico", http.FileServer(http.Dir("./")))
	WebServeMux.Handle("/manifest.json", http.FileServer(http.Dir("./html/")))

	WebServeMux.HandleFunc("/r/iceconfig", iceconfigPageHandler)
	WebServeMux.HandleFunc("/r/iceconfig/", iceconfigPageHandler)
	WebServeMux.HandleFunc("/r/", roomPageHandler)
	WebServeMux.HandleFunc("/join/", joinPageHandler)
	WebServeMux.HandleFunc("/leave/", leavePageHandler)
	WebServeMux.HandleFunc("/message/", messagePageHandler)
	WebServeMux.HandleFunc("/params.html", paramsHTMLPageHandler)
	WebServeMux.HandleFunc("/params.htm", paramsHTMLPageHandler)
	WebServeMux.HandleFunc("/params", paramsPageHandler)
	WebServeMux.HandleFunc("/params/", paramsPageHandler)
	WebServeMux.HandleFunc("/a/", aPageHandler)
	WebServeMux.HandleFunc("/compute/", computePageHandler)
	WebServeMux.HandleFunc("/iceconfig", iceconfigPageHandler)
	WebServeMux.HandleFunc("/iceconfig/", iceconfigPageHandler)
	WebServeMux.HandleFunc("/", mainPageHandler)

	// c := collider.NewCollider(*roomSrv)
	c := collider.NewCollider("")
	c.AddHandle(WebServeMux)
	// go c.Run(wssHostPort, wsHostPort, *CERT, *KEY)

	var e error

	if len(*CERT) > 0 && len(*KEY) > 0 {
		config := &tls.Config{
			// Only allow ciphers that support forward secrecy for iOS9 compatibility:
			// https://developer.apple.com/library/prerelease/ios/technotes/App-Transport-Security-Technote/
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			},
			PreferServerCipherSuites: true,
		}
		log.Println("Starting webrtc demo on https address", httpsHostPort)
		server := &http.Server{Addr: httpsHostPort, Handler: WebServeMux, TLSConfig: config}
		go server.ListenAndServeTLS(*CERT, *KEY)
	}

	log.Println("Starting webrtc demo on http address", httpHostPort)
	e = http.ListenAndServe(httpHostPort, WebServeMux)
	if e != nil {
		log.Fatal("Run: " + e.Error())
	}
	// http.ListenAndServe(":8888", nil)

}
