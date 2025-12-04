package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

const viewBoxWidth = 2450

var groupWidths = map[float64]float64{
	50:   920,
	970:  920,
	1890: 520,
}

func main() {
	// Читаємо весь файл одразу для обробки багаторядкових елементів
	data, err := os.ReadFile("full.html")
	if err != nil {
		fmt.Printf("Помилка: %v\n", err)
		return
	}

	content := string(data)

	// Знаходимо всі групи під'їздів і обробляємо їх вміст
	groupRe := regexp.MustCompile(`(?s)<g transform="translate\(([0-9.]+), ([0-9.]+)\)">(.*?)</g>`)

	content = groupRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := groupRe.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}

		offset, _ := strconv.ParseFloat(parts[1], 64)
		groupContent := parts[3]

		width, ok := groupWidths[offset]
		if !ok {
			return match // Залишаємо без змін для невідомих груп
		}

		// Дзеркалимо polygon points (включаючи багаторядкові)
		polygonRe := regexp.MustCompile(`(?s)points="([^"]*)"`)
		groupContent = polygonRe.ReplaceAllStringFunc(groupContent, func(pm string) string {
			pointsMatch := polygonRe.FindStringSubmatch(pm)
			if len(pointsMatch) < 2 {
				return pm
			}

			pointsStr := pointsMatch[1]
			// Витягуємо всі числа
			coordRe := regexp.MustCompile(`([0-9.]+),([0-9.]+)`)

			result := coordRe.ReplaceAllStringFunc(pointsStr, func(coord string) string {
				coordParts := coordRe.FindStringSubmatch(coord)
				if len(coordParts) < 3 {
					return coord
				}
				x, _ := strconv.ParseFloat(coordParts[1], 64)
				y := coordParts[2]
				newX := width - x
				return fmt.Sprintf("%.0f,%s", newX, y)
			})

			return fmt.Sprintf(`points="%s"`, result)
		})

		// Дзеркалимо line x1, x2
		x1Re := regexp.MustCompile(`x1="([0-9.]+)"`)
		groupContent = x1Re.ReplaceAllStringFunc(groupContent, func(m string) string {
			val := x1Re.FindStringSubmatch(m)[1]
			x, _ := strconv.ParseFloat(val, 64)
			return fmt.Sprintf(`x1="%.1f"`, width-x)
		})

		x2Re := regexp.MustCompile(`x2="([0-9.]+)"`)
		groupContent = x2Re.ReplaceAllStringFunc(groupContent, func(m string) string {
			val := x2Re.FindStringSubmatch(m)[1]
			x, _ := strconv.ParseFloat(val, 64)
			return fmt.Sprintf(`x2="%.1f"`, width-x)
		})

		// Дзеркалимо позицію самої групи (translate)
		// Нова позиція = viewBoxWidth - стара_позиція - ширина_групи
		newOffset := viewBoxWidth - offset - width

		// Повертаємо оновлену групу з новим translate
		return fmt.Sprintf(`<g transform="translate(%.0f, %s)">%s</g>`, newOffset, parts[2], groupContent)
	})

	// Дзеркалимо text елементи поза групами (room-numbers)
	textXRe := regexp.MustCompile(`<text\s+x="([0-9.]+)"`)

	// Знаходимо групу room-numbers і обробляємо її окремо
	roomNumbersRe := regexp.MustCompile(`(?s)(<g id="room-numbers">)(.*?)(</g>)`)
	content = roomNumbersRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := roomNumbersRe.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}

		roomContent := parts[2]

		// Дзеркалимо всі text x координати та додаємо text-anchor="end"
		roomContent = textXRe.ReplaceAllStringFunc(roomContent, func(tm string) string {
			val := textXRe.FindStringSubmatch(tm)[1]
			x, _ := strconv.ParseFloat(val, 64)
			return fmt.Sprintf(`<text x="%.0f" text-anchor="end"`, viewBoxWidth-x)
		})

		return parts[1] + roomContent + parts[3]
	})

	// Зберігаємо результат
	err = os.WriteFile("mirror.html", []byte(content), 0644)
	if err != nil {
		fmt.Printf("Помилка запису: %v\n", err)
		return
	}

	fmt.Println("✅ Успішно створено mirror.html!")
	fmt.Println("   - Polygon контури дзеркально відображені")
	fmt.Println("   - Всі лінії перераховані")
	fmt.Println("   - Текстові підписи в правильних позиціях")
}
