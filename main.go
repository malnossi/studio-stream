package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"studio-stream/internal/audio"
	"studio-stream/internal/engine"
	"studio-stream/internal/streamer"

	"github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

//go:embed assets/Roboto-Regular.ttf
var robotoFontData []byte

type StreamProfile struct {
	Name            string `json:"name"`
	Host            string `json:"host"`
	Port            string `json:"port"`
	UseTLS          bool   `json:"useTLS"`
	MountPoint      string `json:"mountPoint"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	StreamName      string `json:"streamName"`
	Description     string `json:"description"`
	Genre           string `json:"genre"`
	DeviceName      string `json:"deviceName"`
	CodecIndex      int32  `json:"codecIndex"`
	SampleRateIndex int32  `json:"sampleRateIndex"`
	BitrateIndex    int32  `json:"bitrateIndex"`
	ChannelsIndex   int32  `json:"channelsIndex"`
}

const configFileName = "configs.json"

func loadProfiles() []StreamProfile {
	data, err := os.ReadFile(configFileName)
	if err != nil {
		return []StreamProfile{defaultProfile()}
	}
	var profiles []StreamProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return []StreamProfile{defaultProfile()}
	}
	if len(profiles) == 0 {
		return []StreamProfile{defaultProfile()}
	}
	return profiles
}

func saveProfiles(profiles []StreamProfile) {
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err == nil {
		os.WriteFile(configFileName, data, 0644)
	}
}

func defaultProfile() StreamProfile {
	return StreamProfile{
		Name:            "Default Profile",
		Host:            "localhost",
		Port:            "8000",
		UseTLS:          false,
		MountPoint:      "/live",
		Username:        "source",
		Password:        "hackme",
		StreamName:      "StudioStream",
		Description:     "Live Audio Broadcast",
		Genre:           "Misc",
		CodecIndex:      0,
		SampleRateIndex: 0,
		BitrateIndex:    1,
		ChannelsIndex:   1,
	}
}

// UI States
const (
	ViewMain = iota
	ViewConfig
)

func main() {
	if err := audio.Initialize(); err != nil {
		log.Fatalf("PortAudio Init Error: %v", err)
	}
	defer audio.Terminate()

	rl.InitWindow(460, 600, "StudioStream")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)

	// Reset to light mode default
	raygui.LoadStyleDefault()
	raygui.SetStyle(raygui.DEFAULT, raygui.TEXT_SIZE, 14)
	raygui.SetStyle(raygui.DROPDOWNBOX, raygui.TEXT_PADDING, 4)
	raygui.SetStyle(raygui.DROPDOWNBOX, raygui.TEXT_ALIGNMENT, raygui.TEXT_ALIGN_LEFT)
	raygui.SetStyle(raygui.DEFAULT, raygui.TEXT_SPACING, 1)

	// Load Roboto Font from embedded memory
	font := rl.LoadFontFromMemory(".ttf", robotoFontData, 14, nil)
	raygui.SetFont(font)
	defer rl.UnloadFont(font)

	// Engine Setup
	stateChan := make(chan struct {
		State   engine.EngineState
		Message string
	}, 10)

	eng := engine.NewEngine(func(state engine.EngineState, msg string) {
		select {
		case stateChan <- struct {
			State   engine.EngineState
			Message string
		}{state, msg}:
		default:
		}
	})

	devicesSeq, err := audio.GetInputDevices()
	if err != nil {
		log.Fatalf("Failed to get audio devices: %v", err)
	}
	devices := slices.Collect(devicesSeq)
	deviceNamesArr := make([]string, len(devices))
	for i, dev := range devices {
		deviceNamesArr[i] = dev.Name
	}
	deviceNamesStr := strings.Join(deviceNamesArr, ";")
	if len(devices) == 0 {
		deviceNamesStr = "No devices found"
	}

	profiles := loadProfiles()
	currentProfileIndex := int32(0)
	var activeProfile *StreamProfile = &profiles[0]

	// UI State vars for active config
	profName := activeProfile.Name
	host := activeProfile.Host
	port := activeProfile.Port
	useTLS := activeProfile.UseTLS
	mount := activeProfile.MountPoint
	username := activeProfile.Username
	password := activeProfile.Password
	streamName := activeProfile.StreamName
	description := activeProfile.Description
	genre := activeProfile.Genre

	codecIndex := activeProfile.CodecIndex
	sampleRateIndex := activeProfile.SampleRateIndex
	bitrateIndex := activeProfile.BitrateIndex
	channelsIndex := activeProfile.ChannelsIndex

	activeDeviceIndex := int32(0)
	for i, n := range deviceNamesArr {
		if n == activeProfile.DeviceName {
			activeDeviceIndex = int32(i)
			break
		}
	}

	codecs := "mp3;aac;ogg;opus"
	sampleRates := "44100;48000"
	sampleRateValues := []int{44100, 48000}
	bitrates := "128;192;256;320"
	bitrateValues := []int{128, 192, 256, 320}
	channelsList := "Mono;Stereo"
	channelsValues := []int{1, 2}

	// Edit states
	nameEdit, hostEdit, portEdit, mountEdit, userEdit, passEdit := false, false, false, false, false, false
	sNameEdit, descEdit, genreEdit := false, false, false
	deviceEditMode, codecEditMode, srEditMode, brEditMode, chEditMode, profileSelMode := false, false, false, false, false, false

	viewState := ViewMain

	currentEngineState := engine.StateDisconnected
	statusMessage := "Ready"

	closeOtherDropdowns := func(except *bool) {
		if except != &deviceEditMode {
			deviceEditMode = false
		}
		if except != &codecEditMode {
			codecEditMode = false
		}
		if except != &srEditMode {
			srEditMode = false
		}
		if except != &brEditMode {
			brEditMode = false
		}
		if except != &chEditMode {
			chEditMode = false
		}
		if except != &profileSelMode {
			profileSelMode = false
		}
	}

	updateUIFromProfile := func(idx int32) {
		if int(idx) < len(profiles) {
			p := profiles[idx]
			profName = p.Name
			host = p.Host
			port = p.Port
			useTLS = p.UseTLS
			mount = p.MountPoint
			username = p.Username
			password = p.Password
			streamName = p.StreamName
			description = p.Description
			genre = p.Genre
			codecIndex = p.CodecIndex
			sampleRateIndex = p.SampleRateIndex
			bitrateIndex = p.BitrateIndex
			channelsIndex = p.ChannelsIndex
			activeDeviceIndex = 0
			for i, n := range deviceNamesArr {
				if n == p.DeviceName {
					activeDeviceIndex = int32(i)
					break
				}
			}
		}
	}

	saveUIIntoProfile := func(idx int32) {
		cleanStr := func(s string) string {
			return strings.TrimRight(s, "\x00")
		}
		if int(idx) < len(profiles) {
			profiles[idx].Name = cleanStr(profName)
			profiles[idx].Host = cleanStr(host)
			profiles[idx].Port = cleanStr(port)
			profiles[idx].UseTLS = useTLS
			profiles[idx].MountPoint = cleanStr(mount)
			profiles[idx].Username = cleanStr(username)
			profiles[idx].Password = cleanStr(password)
			profiles[idx].StreamName = cleanStr(streamName)
			profiles[idx].Description = cleanStr(description)
			profiles[idx].Genre = cleanStr(genre)
			profiles[idx].CodecIndex = codecIndex
			profiles[idx].SampleRateIndex = sampleRateIndex
			profiles[idx].BitrateIndex = bitrateIndex
			profiles[idx].ChannelsIndex = channelsIndex
			if int(activeDeviceIndex) < len(deviceNamesArr) {
				profiles[idx].DeviceName = deviceNamesArr[activeDeviceIndex]
			}
		}
	}

	for !rl.WindowShouldClose() {
		select {
		case s := <-stateChan:
			currentEngineState = s.State
			statusMessage = s.Message
		default:
		}

		dropdownActive := deviceEditMode || codecEditMode || srEditMode || brEditMode || chEditMode || profileSelMode

		rl.BeginDrawing()
		rl.ClearBackground(rl.GetColor(uint(0xF8F9FAFF)))

		if viewState == ViewMain {
			// ==========================================
			// MAIN VIEW
			// ==========================================
			rl.DrawRectangle(0, 0, 460, 40, rl.GetColor(uint(0x2B8EADFF)))
			rl.DrawTextEx(font, "StudioStream", rl.Vector2{X: 15, Y: 10}, 18, 1, rl.White)

			if dropdownActive {
				raygui.Lock()
			}

			// Top: VU Meter Area (very large)
			rl.DrawRectangle(15, 60, 430, 200, rl.GetColor(uint(0xEAEAEAFF)))
			rl.DrawTextEx(font, "AUDIO LEVELS", rl.Vector2{X: 25, Y: 70}, 14, 1, rl.Gray)

			left, right := float64(0), float64(0)
			if currentEngineState != engine.StateDisconnected {
				left, right = eng.GetLevels()
			}

			// Draw nice big VU meters
			barWidth := int32(410)
			rl.DrawRectangle(25, 100, int32(left*float64(barWidth)), 50, rl.GetColor(uint(0x27AE60FF)))
			rl.DrawRectangleLines(25, 100, barWidth, 50, rl.DarkGray)
			rl.DrawRectangle(25, 170, int32(right*float64(barWidth)), 50, rl.GetColor(uint(0x27AE60FF)))
			rl.DrawRectangleLines(25, 170, barWidth, 50, rl.DarkGray)

			// Config Selection
			y := float32(290)
			raygui.Label(rl.NewRectangle(15, y, 100, 30), "Broadcast Profile:")

			// Configuration Button
			if raygui.Button(rl.NewRectangle(345, y, 100, 30), "Edit Configs") {
				viewState = ViewConfig
			}

			// Giant Start/Stop Broadcast Button
			btnY := float32(370)

			if currentEngineState == engine.StateDisconnected {
				// Green Button
				raygui.SetStyle(raygui.DEFAULT, raygui.BASE_COLOR_NORMAL, 0x27AE60FF)
				raygui.SetStyle(raygui.DEFAULT, raygui.TEXT_COLOR_NORMAL, 0xFFFFFFFF)
				if !dropdownActive && raygui.Button(rl.NewRectangle(15, btnY, 430, 100), "START BROADCAST") {
					codecStr := "mp3"
					switch codecIndex {
					case 1:
						codecStr = "aac"
					case 2:
						codecStr = "ogg"
					case 3:
						codecStr = "opus"
					}

					cleanStr := func(s string) string {
						return strings.TrimSpace(strings.TrimRight(s, "\x00"))
					}

					cfg := streamer.Config{
						Host:        fmt.Sprintf("%s:%s", cleanStr(host), cleanStr(port)),
						UseTLS:      useTLS,
						MountPoint:  cleanStr(mount),
						Username:    cleanStr(username),
						Password:    cleanStr(password),
						StreamName:  cleanStr(streamName),
						Description: cleanStr(description),
						Genre:       cleanStr(genre),
						URL:         "http://studiostream.local",
						Public:      true,
						Bitrate:     bitrateValues[bitrateIndex],
						SampleRate:  sampleRateValues[sampleRateIndex],
						Channels:    channelsValues[channelsIndex],
						Codec:       codecStr,
					}
					eng.StartStream(int(activeDeviceIndex), cfg)
				}
			} else {
				// Red Button
				raygui.SetStyle(raygui.DEFAULT, raygui.BASE_COLOR_NORMAL, 0xE74C3CFF)
				raygui.SetStyle(raygui.DEFAULT, raygui.TEXT_COLOR_NORMAL, 0xFFFFFFFF)
				if !dropdownActive && raygui.Button(rl.NewRectangle(15, btnY, 430, 100), "STOP BROADCAST") {
					eng.StopStream()
				}
			}
			// Reset button style
			raygui.SetStyle(raygui.DEFAULT, raygui.BASE_COLOR_NORMAL, 0xF5F5F5FF)
			raygui.SetStyle(raygui.DEFAULT, raygui.TEXT_COLOR_NORMAL, 0x686868FF)

			// Stats
			if currentEngineState != engine.StateDisconnected {
				stats := eng.GetStats()
				rl.DrawTextEx(font, fmt.Sprintf("Streaming... %d KB Sent | %.1fs Uptime", stats.BytesSent/1024, stats.Uptime), rl.Vector2{X: 15, Y: btnY + 120}, 16, 1, rl.DarkGray)
			} else {
				rl.DrawTextEx(font, fmt.Sprintf("Status: %s", statusMessage), rl.Vector2{X: 15, Y: btnY + 120}, 16, 1, rl.Gray)
			}

			if dropdownActive {
				raygui.Unlock()
			}

			// Profile Dropdown (drawn last so it opens cleanly)
			if dropdownActive && !profileSelMode {
				raygui.Lock()
			} else {
				raygui.Unlock()
			}
			profNamesArr := make([]string, len(profiles))
			for i, p := range profiles {
				profNamesArr[i] = p.Name
			}
			profNamesStr := strings.Join(profNamesArr, ";")

			prevIdx := currentProfileIndex
			if raygui.DropdownBox(rl.NewRectangle(130, y, 205, 30), profNamesStr, &currentProfileIndex, profileSelMode) {
				profileSelMode = !profileSelMode
				if profileSelMode {
					closeOtherDropdowns(&profileSelMode)
				}
			}
			if currentProfileIndex != prevIdx {
				updateUIFromProfile(currentProfileIndex)
			}

		} else {
			// ==========================================
			// CONFIG VIEW
			// ==========================================
			rl.DrawRectangle(0, 0, 460, 40, rl.GetColor(uint(0x34495EFF)))
			rl.DrawTextEx(font, "Configuration Manager", rl.Vector2{X: 15, Y: 10}, 18, 1, rl.White)

			if dropdownActive {
				raygui.Lock()
			}

			// Top actions
			y := float32(50)
			if raygui.Button(rl.NewRectangle(15, y, 80, 25), "<- Back") {
				saveUIIntoProfile(currentProfileIndex)
				saveProfiles(profiles)
				viewState = ViewMain
			}

			if raygui.Button(rl.NewRectangle(280, y, 70, 25), "New") {
				profiles = append(profiles, defaultProfile())
				profiles[len(profiles)-1].Name = fmt.Sprintf("Profile %d", len(profiles))
				currentProfileIndex = int32(len(profiles) - 1)
				updateUIFromProfile(currentProfileIndex)
			}

			if raygui.Button(rl.NewRectangle(360, y, 85, 25), "Delete") {
				if len(profiles) > 1 {
					profiles = append(profiles[:currentProfileIndex], profiles[currentProfileIndex+1:]...)
					currentProfileIndex = 0
					updateUIFromProfile(currentProfileIndex)
				}
			}

			y += 40
			raygui.Label(rl.NewRectangle(15, y, 60, 25), "Name")
			if raygui.TextBox(rl.NewRectangle(70, y, 375, 25), &profName, 64, nameEdit) {
				nameEdit = !nameEdit
			}

			y += 35
			raygui.Label(rl.NewRectangle(15, y, 40, 25), "Host")
			if raygui.TextBox(rl.NewRectangle(15, y+20, 140, 25), &host, 64, hostEdit) {
				hostEdit = !hostEdit
			}
			raygui.Label(rl.NewRectangle(165, y, 40, 25), "Port")
			if raygui.TextBox(rl.NewRectangle(165, y+20, 60, 25), &port, 64, portEdit) {
				portEdit = !portEdit
			}
			useTLSClicked := raygui.CheckBox(rl.NewRectangle(240, y+25, 15, 15), "Use TLS", &useTLS)
			_ = useTLSClicked
			
			raygui.Label(rl.NewRectangle(320, y, 60, 25), "Mount")
			if raygui.TextBox(rl.NewRectangle(320, y+20, 125, 25), &mount, 64, mountEdit) {
				mountEdit = !mountEdit
			}

			y += 50
			raygui.Label(rl.NewRectangle(15, y, 60, 25), "Username")
			if raygui.TextBox(rl.NewRectangle(15, y+20, 205, 25), &username, 64, userEdit) {
				userEdit = !userEdit
			}
			raygui.Label(rl.NewRectangle(240, y, 60, 25), "Password")
			if raygui.TextBox(rl.NewRectangle(240, y+20, 205, 25), &password, 64, passEdit) {
				passEdit = !passEdit
			}

			y += 50
			raygui.Label(rl.NewRectangle(15, y, 100, 25), "Stream Name")
			if raygui.TextBox(rl.NewRectangle(15, y+20, 430, 25), &streamName, 64, sNameEdit) {
				sNameEdit = !sNameEdit
			}

			y += 50
			raygui.Label(rl.NewRectangle(15, y, 100, 25), "Description")
			if raygui.TextBox(rl.NewRectangle(15, y+20, 430, 25), &description, 64, descEdit) {
				descEdit = !descEdit
			}

			y += 50
			raygui.Label(rl.NewRectangle(15, y, 100, 25), "Genre")
			if raygui.TextBox(rl.NewRectangle(15, y+20, 430, 25), &genre, 64, genreEdit) {
				genreEdit = !genreEdit
			}

			if dropdownActive {
				raygui.Unlock()
			}

			// DROPDOWNS for Audio config (drawn bottom to top)
			yDropRow := float32(400)
			colW := float32(100)
			gap := float32(10)

			yDev := yDropRow + 50
			if dropdownActive && !deviceEditMode {
				raygui.Lock()
			} else {
				raygui.Unlock()
			}
			raygui.Label(rl.NewRectangle(15, yDev-20, 100, 25), "Input Device")
			if raygui.DropdownBox(rl.NewRectangle(15, yDev, 430, 25), deviceNamesStr, &activeDeviceIndex, deviceEditMode) {
				deviceEditMode = !deviceEditMode
				if deviceEditMode {
					closeOtherDropdowns(&deviceEditMode)
				}
			}

			if dropdownActive && !chEditMode {
				raygui.Lock()
			} else {
				raygui.Unlock()
			}
			raygui.Label(rl.NewRectangle(15, yDropRow-20, colW, 25), "Channels")
			if raygui.DropdownBox(rl.NewRectangle(15, yDropRow, colW, 25), channelsList, &channelsIndex, chEditMode) {
				chEditMode = !chEditMode
				if chEditMode {
					closeOtherDropdowns(&chEditMode)
				}
			}

			if dropdownActive && !brEditMode {
				raygui.Lock()
			} else {
				raygui.Unlock()
			}
			raygui.Label(rl.NewRectangle(15+(colW+gap)*1, yDropRow-20, colW, 25), "Bitrate")
			if raygui.DropdownBox(rl.NewRectangle(15+(colW+gap)*1, yDropRow, colW, 25), bitrates, &bitrateIndex, brEditMode) {
				brEditMode = !brEditMode
				if brEditMode {
					closeOtherDropdowns(&brEditMode)
				}
			}

			if dropdownActive && !srEditMode {
				raygui.Lock()
			} else {
				raygui.Unlock()
			}
			raygui.Label(rl.NewRectangle(15+(colW+gap)*2, yDropRow-20, colW, 25), "Sample Rate")
			if raygui.DropdownBox(rl.NewRectangle(15+(colW+gap)*2, yDropRow, colW, 25), sampleRates, &sampleRateIndex, srEditMode) {
				srEditMode = !srEditMode
				if srEditMode {
					closeOtherDropdowns(&srEditMode)
				}
			}

			if dropdownActive && !codecEditMode {
				raygui.Lock()
			} else {
				raygui.Unlock()
			}
			raygui.Label(rl.NewRectangle(15+(colW+gap)*3, yDropRow-20, colW, 25), "Codec")
			if raygui.DropdownBox(rl.NewRectangle(15+(colW+gap)*3, yDropRow, colW, 25), codecs, &codecIndex, codecEditMode) {
				codecEditMode = !codecEditMode
				if codecEditMode {
					closeOtherDropdowns(&codecEditMode)
				}
			}

			// Profile dropdown
			yProf := float32(50)
			if dropdownActive && !profileSelMode {
				raygui.Lock()
			} else {
				raygui.Unlock()
			}
			profNamesArr := make([]string, len(profiles))
			for i, p := range profiles {
				profNamesArr[i] = p.Name
			}
			profNamesStr := strings.Join(profNamesArr, ";")

			prevIdx := currentProfileIndex
			if raygui.DropdownBox(rl.NewRectangle(105, yProf, 165, 25), profNamesStr, &currentProfileIndex, profileSelMode) {
				profileSelMode = !profileSelMode
				if profileSelMode {
					closeOtherDropdowns(&profileSelMode)
				}
			}
			if currentProfileIndex != prevIdx {
				// Save current before switching
				saveUIIntoProfile(prevIdx)
				updateUIFromProfile(currentProfileIndex)
			}
		}

		rl.EndDrawing()
		time.Sleep(time.Millisecond * 10)
	}
}
