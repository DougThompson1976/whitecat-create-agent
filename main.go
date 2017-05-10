/*
 * Whitecat Blocky Environment, agent main program
 *
 * Copyright (C) 2015 - 2016
 * IBEROXARXA SERVICIOS INTEGRALES, S.L.
 *
 * Author: Jaume Olivé (jolive@iberoxarxa.com / jolive@whitecatboard.org)
 *
 * All rights reserved.
 *
 * Permission to use, copy, modify, and distribute this software
 * and its documentation for any purpose and without fee is hereby
 * granted, provided that the above copyright notice appear in all
 * copies and that both that the copyright notice and this
 * permission notice and warranty disclaimer appear in supporting
 * documentation, and that the name of the author not be used in
 * advertising or publicity pertaining to distribution of the
 * software without specific, written prior permission.
 *
 * The author disclaim all warranties with regard to this
 * software, including all implied warranties of merchantability
 * and fitness.  In no event shall the author be liable for any
 * special, indirect or consequential damages or any damages
 * whatsoever resulting from loss of use, data or profits, whether
 * in an action of contract, negligence or other tortious action,
 * arising out of or in connection with the use or performance of
 * this software.
 */

package main

import (
	"fmt"
	"github.com/mikepb/go-serial"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"
	"os/user"
	"path"
	"runtime"
	"path/filepath"
)

var Version string = "1.2"

var Upgrading bool = false
var StopMonitor bool = false

var AppFolder = "/"
var AppDataFolder string = "/"
var AppDataTmpFolder string = "/tmp"
var AppFileName = ""

// Connected board
var connectedBoard *Board = nil

// Monitor serial ports and search for a board compatible with Lua RTOS.
// If a board is found, monitors that port continues open over time.
func monitorSerialPorts(devices []deviceDef) {
	log.Println("start monitoring serial ports ...")

	notify("boardUpdate", "Scanning boards")

	for {
		if Upgrading {
			time.Sleep(time.Millisecond * 500)
			continue
		}

		if StopMonitor {
			log.Println("Stop monitoring serial ports ...")
			break
		}

		// If a board is connected ...
		if connectedBoard != nil {
			// Test that port continues open
			_, err := connectedBoard.port.InputWaiting()
			if err != nil {
				notify("boardDetached", "")

				// Port is not open, waiting for a board
				connectedBoard = nil
			} else {
				// Port is open, continue
				time.Sleep(time.Millisecond * 10)
				continue
			}
		}

		// Enumerate all serial ports
		ports, err := serial.ListPorts()
		if err != nil {
			continue
		}

		// Search a serial port that matches with one of the supported adapters
		for _, info := range ports {
			// Read VID/PID
			vendorId, productId, err := info.USBVIDPID()
			if err != nil {
				continue
			}

			// We need a VID / PID
			if vendorId != 0 && productId != 0 {
				vendorId := "0x" + strconv.FormatInt(int64(vendorId), 16)
				productId := "0x" + strconv.FormatInt(int64(productId), 16)

				log.Printf("found adapter, VID %s:%s", vendorId, productId)

				// Search a VID/PIN into requested devices
				for _, device := range devices {
					if device.VendorId == vendorId && device.ProductId == productId {
						// This adapter matches

						log.Printf("check adapter, VID %s:%s", device.VendorId, device.ProductId)

						// Create a candidate board
						var candidate Board

						// Attach candidate
						if candidate.attach(info) {
							break
						}
					}
				}
			}
		}

		time.Sleep(time.Millisecond * 10)
	}
}

func main() {
	withLogFile := false
	withLog := false
	daemon := false
	service := false

	// Get arguments and process arguments
	for _, arg := range os.Args {
		switch arg {
		case "-lf":
			withLogFile = true
		case "-l":
			withLog = true
		case "-s":
			service = true
		case "-d":
			daemon = true
		case "-r":
			time.Sleep(2000 * time.Millisecond)
		case "-v":
			fmt.Println(Version)
			os.Exit(0)
		}
	}

	AppFolder, _ = filepath.Abs(filepath.Dir(os.Args[0]))	
	AppFileName, _ = filepath.Abs(os.Args[0])

	// Get home directory, create the user data folder, and needed folders
	usr, err := user.Current()
	if err != nil {
		os.Exit(1)
	}

	if (runtime.GOOS == "darwin") {
		AppDataFolder = path.Join(usr.HomeDir, "Library", "Application Support", "The Whitecat Create Agent")
	} else if (runtime.GOOS == "windows") {
		AppDataFolder = path.Join(usr.HomeDir, "AppData", "The Whitecat Create Agent")
	}

	AppDataTmpFolder = path.Join(AppDataFolder, "tmp")
	
	_ = os.Mkdir(AppDataFolder, 0755)
	_ = os.Mkdir(AppDataTmpFolder, 0755)

	if withLog || withLogFile {
		// User wants log, so we don't want to execute as daemon
		
		if !withLogFile {
			// Discard all output, so log is not needed
			log.SetOutput(ioutil.Discard)
		} else {
			// User wants log to file, so we don't want to execute as daemon
			f,_ := os.OpenFile(path.Join(AppDataFolder,"log.txt"), os.O_RDWR | os.O_CREATE, 0755)
			log.SetOutput(f)
			defer f.Close()
		}

		exitChan := make(chan int)

		go webSocketStart(exitChan)
		<-exitChan

		os.Exit(0)
	}
	
	if service {
		setupSysTray()
	} else {
		if !daemon {
			// Respawn
			cmd := exec.Command(AppFileName, "-d")
			cmd.Start()
			os.Exit(0)
		} else {
			// This is the spawn process
			setupSysTray()
		}
	}
}
