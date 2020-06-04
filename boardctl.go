
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"net/url"
	"time"
	"log"
	"github.com/gorilla/websocket"
	"github.com/VividCortex/ewma"
	"encoding/json"
)

type status struct {
	Version                                    string
	Build                                      string
	HardwareBuild                              string
	Devices                                    uint32
	Connected_Users                            uint
	DiskBytesFree                              uint64
	UAT_messages_last_minute                   uint
	UAT_messages_max                           uint
	ES_messages_last_minute                    uint
	ES_messages_max                            uint
	UAT_traffic_targets_tracking               uint16
	ES_traffic_targets_tracking                uint16
	Ping_connected                             bool
	GPS_satellites_locked                      uint16
	GPS_satellites_seen                        uint16
	GPS_satellites_tracked                     uint16
	GPS_position_accuracy                      float32
	GPS_connected                              bool
	GPS_solution                               string
	RY835AI_connected                          bool
	Uptime                                     int64
	UptimeClock                                time.Time
	CPUTemp                                    float32
	NetworkDataMessagesSent                    uint64
	NetworkDataMessagesSentNonqueueable        uint64
	NetworkDataBytesSent                       uint64
	NetworkDataBytesSentNonqueueable           uint64
	NetworkDataMessagesSentLastSec             uint64
	NetworkDataMessagesSentNonqueueableLastSec uint64
	NetworkDataBytesSentLastSec                uint64
	NetworkDataBytesSentNonqueueableLastSec    uint64
	UAT_METAR_total                            uint32
	UAT_TAF_total                              uint32
	UAT_NEXRAD_total                           uint32
	UAT_SIGMET_total                           uint32
	UAT_PIREP_total                            uint32
	UAT_NOTAM_total                            uint32
	UAT_OTHER_total                            uint32

	Errors []string
}

var addr = flag.String("addr", "localhost", "http service address")

var chanDone chan bool

var chanPWR chan int
var chanGPS chan int
var chanADS chan int
var chanFAN chan int

var pinFAN2 = 18
var pinFAN = 13
var pinLED = 12
var pinPWR = 26
var pinGPS = 6
var pinADS = 5

var fanmode = 0
var tempAvg = ewma.NewMovingAverage(5)

func writeCommand(pin int, pwm float32) {

	var cmd = fmt.Sprintf("%d=%.02f\n", pin, pwm)
	//fmt.Println(cmd)

	file, err := os.OpenFile("/dev/pi-blaster", os.O_WRONLY|os.O_TRUNC|os.O_CREATE,0666) // For write access.
	if err != nil {
		log.Fatal(err)
	}
    _, err = file.Write([]byte(cmd))
    if err != nil {
        log.Fatal(err)
    }
    file.Sync()
    file.Close()
}


func ledsOn() {
	writeCommand(pinPWR, 1)
	writeCommand(pinGPS, 1)
	writeCommand(pinADS, 1)
}

func ledsOff() {
	writeCommand(pinPWR, 0)
	writeCommand(pinGPS, 0)
	writeCommand(pinADS, 0)
}

func controlPower() {
	// States: off (off = 0), error (slow blink = 1), on/ok (solid = 2)

	var count = 0
	var mode = 0
	writeCommand(pinPWR, 0)

	for {
		timeChanP := time.NewTimer(time.Millisecond * 500).C
		select {
		case data := <-chanPWR:
			mode = data
		case <- timeChanP:
			if (mode == 0) {
				writeCommand(pinPWR, 0)
			} else
			if (mode == 2) {
				writeCommand(pinPWR, 1)
			} else
			if (mode == 1) {
				if (count == 0) {
					count = 1
					writeCommand(pinPWR, 1)
				} else {
					count = 0
					writeCommand(pinPWR, 0)
				}
			}
		}
	}
}

func controlGPS() {
	// States: disconnected (off = 0), no fix (slow blink = 1), fix (solid = 2)

	var count = 0
	var mode = 0

	writeCommand(pinGPS, 0)

	for {
		timeChanG := time.NewTimer(time.Millisecond * 500).C
		select {
		case data := <-chanGPS:
			mode = data
		case <- timeChanG:
			if (mode == 0) {
				writeCommand(pinGPS, 0)
			} else
			if (mode == 2) {
				writeCommand(pinGPS, 1)
			} else
			if (mode == 1) {
				if (count == 0) {
					count = 1
					writeCommand(pinGPS, 1)
				} else {
					count = 0
					writeCommand(pinGPS, 0)
				}
			}
		}
	}
}

func controlADSB() {
	// States: no data (off = 0), data on one channel (slow blink = 1), data on both (solid = 2)

	var count = 0
	var mode = 0
	writeCommand(pinADS, 0)

	for {
		timeChanA := time.NewTimer(time.Millisecond * 500).C
		select {
		case data := <-chanADS:
			mode = data
		case <- timeChanA:
			if (mode == 0) {
				writeCommand(pinADS, 0)
			} else
			if (mode == 2) {
				writeCommand(pinADS, 1)
			} else
			if (mode == 1) {
				if (count == 0) {
					count = 1
					writeCommand(pinADS, 1)
				} else {
					count = 0
					writeCommand(pinADS, 0)
				}
			}
		}
	}
}

func indicateStratuxDown() {
	writeCommand(pinGPS, 0)
	writeCommand(pinADS, 0)
	writeCommand(pinFAN, 1)
	writeCommand(pinFAN2, 1)
	chanGPS <- 0
	chanADS <- 0
	chanPWR <- 1
}

func listenOnWebsocket() {

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/status"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("Unable to connect to WebSocket: ", err)
		indicateStratuxDown()
		chanDone <- true
		return
	}
	
	defer c.Close()
	
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			//TODO: if isCloseError isUnexpectedCloseError this daemon should begin reconnect attempt
			log.Println("WebSocket closed:", err)
			indicateStratuxDown()
			chanDone <- true
			return
		}

		res := status{}
		json.Unmarshal([]byte(message), &res)

		// check CPU temperature
		tempAvg.Add(float64(res.CPUTemp))
		temp := tempAvg.Value()
		if (fanmode == 0) {
			if (temp >= 40) {
				fanmode = 1
				writeCommand(pinFAN, 0.5)
				writeCommand(pinFAN2, 0.5)
			} else if (temp >= 50) {
				fanmode = 2
				writeCommand(pinFAN, 1)
				writeCommand(pinFAN2, 1)
			} else {
				// do nothing - 0 / 0 is good
			}
		} else if (fanmode == 1) {
			if (temp <= 35) {
				fanmode = 0
				writeCommand(pinFAN, 0)
				writeCommand(pinFAN2, 0)
			} else if (temp >= 50) {
				fanmode = 2
				writeCommand(pinFAN, 1)
				writeCommand(pinFAN2, 1)
			} else {
				// do nothing = 1 / 0.5 is good
			}
		} else if (fanmode == 2) {
			if (temp <= 45) {
				fanmode = 1
				writeCommand(pinFAN, 0.5)
				writeCommand(pinFAN2, 0.5)
			} else if (temp <= 35) {
				fanmode = 0
				writeCommand(pinFAN, 0)
				writeCommand(pinFAN2, 0)
			} else {
				// do nothing = 2 / 1 is good
			}
		}
		log.Printf("Temp: %0.2f - Mode: %d\n", temp, fanmode)

		// check the GPS status
		if (res.GPS_solution == "Disconnected") {
			chanGPS <- 0 // off
		} else
		if (res.GPS_solution == "No Fix") {
			chanGPS <- 1 // blink
		} else {
			chanGPS <- 2 // solid
		}

		// check the number of ADS-B messages we're receiving
		if ((res.UAT_messages_last_minute > 0) && (res.ES_messages_last_minute > 1)) {
			chanADS <- 2
		} else
		if ((res.UAT_messages_last_minute == 0) && (res.ES_messages_last_minute > 1)) {
			chanADS <- 1
		} else
		if ((res.UAT_messages_last_minute > 0) && (res.ES_messages_last_minute <= 1)) {
			chanADS <- 1
		} else {
			chanADS <- 0
		}

		chanPWR <- 2

	}

}

func main() {

	flag.Parse()
	log.SetFlags(0)

	xchan := make(chan os.Signal, 1)
	signal.Notify(xchan, os.Interrupt)
	go func(){
		for range xchan {
			// sig is a ^C, handle it
			chanPWR <- -1
			chanGPS <- -1
			chanADS <- -1
			ledsOff()
			writeCommand(pinFAN, 0)
			writeCommand(pinFAN2, 0)
			os.Exit(1)
		}
	}()

	// init channels
	chanPWR = make(chan int)
	chanGPS = make(chan int)
	chanADS = make(chan int)

	// start channels listening for commands
	go controlPower()
	go controlGPS()
	go controlADSB()


	// turn on the LEDs (brightness line)
	ledsOn()

	// turn off the fan
	writeCommand(pinFAN, 0)
	writeCommand(pinFAN2, 0)

	// flash the startup sequence
	time.Sleep(time.Millisecond * 500)
	writeCommand(pinLED, 0)
	time.Sleep(time.Millisecond * 500)
	writeCommand(pinLED, 1)
	time.Sleep(time.Millisecond * 500)
	writeCommand(pinLED, 0)
	time.Sleep(time.Millisecond * 500)
	writeCommand(pinLED, 1)
	time.Sleep(time.Millisecond * 500)
	writeCommand(pinLED, 0)

	// turn off individual LEDs
	ledsOff()

	// set the LED master line high (will eventually be PWM)
	writeCommand(pinLED, 1)

	// create the "socket disconnect" channel
	chanDone = make(chan bool)
	defer close(chanDone)
	
	go listenOnWebsocket()

	for {
		select {
		case <- chanDone:
			log.Printf("Socket disconnected / failed to connect")
			time.Sleep(time.Second * 1)
			go listenOnWebsocket()
		}
	}
}
