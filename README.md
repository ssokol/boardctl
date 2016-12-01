# boardctl
Control service for AHRS / GPS / Baro board - runs the LEDs and fan

Requires pi-blaster for PWM

Use my fork of pi-blaster - it enables pin 6 which was banned in the
original version:

https://github.com/ssokol/pi-blaster

First, build pi-blaster following the really simple instructions:

```
./autogen.sh
./configure
make
sudo make install
```

Next, copy the pin configuration file to /etc/default:

```
sudo cp pi-blaster.conf /etc/default/pi-blaster
sudo systemctl restart pi-blaster
```

Now build the board control program:

```
cd ../boardctl
go get github.com/gorilla/websocket
go build boardctl.go
sudo cp boardctl /usr/bin/
```

Now enable the service:

```
sudo cp boardctl.service /lib/systemd/system/
sudo systemctl enable boardctl
sudo systemctl start boardctl
```

Congrats. You should now have blinkenlights and buzzinfan.

What the lights mean:

Green = power (takes about 20 seconds after power-on for this one to light up)

Orange = GPS status
  * off = disconnected
  * blink = no lock
  * solid = lock

Blue = ADS-B status
  * off = no signals (either band)
  * blink = receiving on one band (usually 1090)
  * solid = receiving on both bands
  
For building a Stratux / FlightBox distro or update you will need to copy:

1. /usr/sbin/pi-blaster
1. /lib/systemd/system/pi-blaster.service
1. /etc/default/pi-blaster
1. /usr/sbin/boardctl
1. /lib/systemd/system/boardctl.service
  