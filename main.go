package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	cfg, needSetup := loadConfig()

	if needSetup {
		fmt.Println("╔══════════════════════════════════════╗")
		fmt.Println("║       LOLZCHAT — Первый запуск       ║")
		fmt.Println("╚══════════════════════════════════════╝")
		fmt.Println()
		fmt.Println("Получи токен: https://lolz.live/account/api")
		fmt.Println()

		for {
			fmt.Print("Введи токен: ")
			if !scanner.Scan() {
				os.Exit(0)
			}
			token := strings.TrimSpace(scanner.Text())
			if token == "" {
				fmt.Println("Токен не может быть пустым!")
				continue
			}

			cfg.Token = token

			fmt.Print("\nПроверяем токен...")
			api := NewAPIClient(cfg.BaseURL, cfg.Token)
			_, name, err := api.GetMe()
			if err != nil {
				fmt.Printf(" Ошибка: %v\n", err)
				fmt.Println("Попробуй другой токен.\n")
				continue
			}
			fmt.Printf(" OK! Привет, %s\n", name)

			if err := saveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Не удалось сохранить конфиг: %v\n", err)
			} else {
				fmt.Printf("Конфиг сохранён: %s\n", configPath())
			}
			break
		}
		fmt.Println()
	}

	api := NewAPIClient(cfg.BaseURL, cfg.Token)

	fmt.Print("Подключение...")
	myID, myName, err := api.GetMe()
	if err != nil {
		fmt.Fprintf(os.Stderr, " Ошибка авторизации: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(" OK! %s (ID: %d)\n\n", myName, myID)

	rooms, err := api.GetRooms()
	if err != nil || len(rooms) == 0 {
		fmt.Fprintf(os.Stderr, "Не удалось получить список комнат: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Доступные комнаты:")
	for i, r := range rooms {
		fmt.Printf("  [%d] #%d — %s\n", i+1, r.RoomID, r.Title)
	}

	var roomID int
	for {
		fmt.Print("\nВыбери комнату (номер): ")
		if !scanner.Scan() {
			os.Exit(0)
		}
		input := strings.TrimSpace(scanner.Text())
		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(rooms) {
			fmt.Printf("Введи число от 1 до %d\n", len(rooms))
			continue
		}
		roomID = rooms[num-1].RoomID
		fmt.Printf("Заходим в #%d — %s...\n", roomID, rooms[num-1].Title)
		break
	}

	roomTitle := ""
	for _, r := range rooms {
		if r.RoomID == roomID {
			roomTitle = r.Title
			break
		}
	}

	m := initialModel(cfg, api, myID, myName, roomID, roomTitle)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
}
