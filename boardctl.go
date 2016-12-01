
package main

import (
	"flag"
	"fmt"
	"os"
//	"io/ioutil"
	"os/signal"
	"net/url"
	"time"
	"log"
	"github.com/gorilla/websocket"
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

var chanPWR chan int
var chanGPS chan int
var chanADS chan int
var chanFAN chan int

var pinFAN = 13
var pinLED = 12
var pinPWR = 26
var pinGPS = 6
var pinADS = 5

func writeCommand(pin int, pwm float32) {

	var cmd = fmt.Sprintf("%d=%.02f\n", pin, pwm)
	fmt.Println(cmd)

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

/*
func controlFan() {

	var count = 0
	var cutoff = 40
	var units = 40

	for {
		timeChanF := time.NewTimer(time.Microsecond * 40).C // 25kHz
		select {
		case speed := <-chanFAN:
			if (speed < 0) {
				return
			} else {
				var f float32 = (float32(speed) / 100)
				cutoff = int(f * 40)
			}
			fmt.Println("cutoff: ", speed, cutoff)
		case <- timeChanF:
			if (count < cutoff) {
				fan.High()
			} else {
				fan.Low()
			}
			count = count + 1
			if (count > (units - 1)) {
				count = 0
			}
		}
	}
}

func controlFan() {
        for {

                // Set up tickers for 60% PWM @ 1 kHz.

                onChanF := time.NewTicker(time.Millisecond * 1).C
                time.Sleep(600 * time.Microsecond)
                offChanF := time.NewTicker(time.Millisecond * 1).C


                select {
                case speed := <-chanFAN:
                        var cutoff int
                        if (speed < 0) {
                                return
                        } else {
                                var f float32 = (float32(speed) / 100)
                                cutoff = int(f * 1000)
                        }
                        fmt.Println("fancutoff: ", speed, cutoff)

                        onChanF = time.NewTicker(time.Millisecond * 1).C
                        time.Sleep(time.Duration(cutoff) * time.Microsecond)
                        offChanF = time.NewTicker(time.Millisecond * 1).C

                case <- onChanF:
                        fan.High()
                case <- offChanF:
                        fan.Low()
                }
        }
}
*/

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
			os.Exit(1)
		}
	}()

	// init channels
	chanPWR = make(chan int)
	chanGPS = make(chan int)
	chanADS = make(chan int)

	// start listening
	//go controlFan()
	go controlPower()
	go controlGPS()
	go controlADSB()

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/status"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}

	defer c.Close()

	done := make(chan struct{})

	// socket listener function
	go func() {
		defer c.Close()
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				//TODO: if isCloseError isUnexpectedCloseError this daemon should begin reconnect attempt
				log.Println("read:", err)
				return
			}

			res := status{}
    		json.Unmarshal([]byte(message), &res)

    		if (res.CPUTemp < 40) {
    			writeCommand(pinFAN, 0)
    		} else {
    			writeCommand(pinFAN, 1)
    		}

			if (res.GPS_solution == "Disconnected") {
				chanGPS <- 0 // off
			} else
			if (res.GPS_solution == "No Fix") {
				chanGPS <- 1 // blink
			} else {
				chanGPS <- 2 // solid
			}

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
	}()

	// turn on the LEDs (brightness line)
	ledsOn()

	// turn off the fan
	writeCommand(pinFAN, 0)

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

	// turn off individual LED
	ledsOff()

	// set the LED master line high (will eventually be PWM)
	writeCommand(pinLED, 1)



	select {}
}