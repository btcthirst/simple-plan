package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/net/html"
)

// Приклад HTML-документа, який ми будемо використовувати як вміст файлу.
// Додано простий SVG-тег для демонстрації.
const exampleHTMLContent = `
<!DOCTYPE html>
<html>
<head>
    <title>Приклад Сторінки, Завантаженої з Файлу</title>
</head>
<body>
    <h1>Ласкаво просимо!</h1>
    
    <!-- Наш цільовий SVG, який ми будемо витягувати -->
    <svg id="chart" width="200" height="100" viewBox="0 0 200 100" xmlns="http://www.w3.org/2000/svg">
        <rect width="100%" height="100%" fill="#FFEECC" />
        <circle cx="50" cy="50" r="40" stroke="#FF5733" stroke-width="3" fill="#FFC300" />
        <text x="100" y="90" font-family="Arial" font-size="10" text-anchor="middle" fill="#333">Тестовий SVG</text>
    </svg>

    <p class="main-content">Цей текст був успішно витягнутий з локального файлу.</p>
    <div id="footer">
        <a href="https://golang.org/">GoLang</a>
        <span>&copy; 2024</span>
    </div>
</body>
</html>
`

const (
	// 1. Визначаємо ім'я вхідного файлу
	inputFilename = "mirror.html" //"full.html" //"plan2.html" //"plan1.html"
	// 1. Визначаємо ім'я вихідного файлу
	targetSVGFilename = "mirror.svg" //"full.svg" //"2.svg" //"1.svg"
	targetPNGFilename = "mirror.png" //"full.png" //"2.png" //"1.png"
)

// ensureFileExists перевіряє, чи існує файл, і якщо ні, створює його з прикладом вмісту.
func ensureFileExists(filename string) error {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		fmt.Printf("Файл %s не знайдено. Створюємо його з тестовим вмістом...\n", filename)
		return os.WriteFile(filename, []byte(exampleHTMLContent), 0644)
	}
	return err // Повертає nil, якщо файл існує, або іншу помилку Stat
}

// parseContent виконує парсинг HTML з довільного io.Reader.
func parseContent(reader io.Reader) (*html.Node, error) {
	// Парсинг HTML
	doc, err := html.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("Помилка парсингу HTML: %v", err)
	}

	fmt.Println("--- Результати парсингу з файлу ---")

	// 2. Обхід дерева DOM для пошуку потрібного елемента (<title>)
	foundTitle := findTag(doc, "title")
	if foundTitle != "" {
		fmt.Printf("Знайдено заголовок сторінки (<title>): %s\n", foundTitle)
	}

	// 3. Пошук елемента за класом 'main-content'
	var paragraphContent string

	searchNode := func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "p" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && attr.Val == "main-content" {
					if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
						paragraphContent = n.FirstChild.Data
						return true
					}
				}
			}
		}
		return false
	}

	traverse(doc, searchNode)

	if paragraphContent != "" {
		fmt.Printf("Знайдено вміст параграфа: %s\n", paragraphContent)
	} else {
		fmt.Println("Елемент з класом 'main-content' не знайдено.")
	}

	return doc, nil
}

// extractAndSaveSVG знаходить перший SVG-елемент у дереві та зберігає його у вказаний файл.
func extractAndSaveSVG(doc *html.Node, outputFilename string) error {
	var svgNode *html.Node

	// Рекурсивна функція пошуку першого SVG-вузла
	var findSVG func(*html.Node)
	findSVG = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "svg" {
			svgNode = n
			return
		}
		// Обхід дочірніх елементів
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			// Якщо ми вже знайшли SVG-вузол, зупиняємо пошук у цій гілці.
			if svgNode != nil {
				return
			}
			findSVG(c)
		}
	}

	findSVG(doc)

	if svgNode == nil {
		return fmt.Errorf("Не вдалося знайти тег <svg> у HTML-документі")
	}

	// Серіалізуємо SVG-вузол з правильним форматуванням для SVG
	var buf bytes.Buffer
	if err := renderSVG(&buf, svgNode); err != nil {
		return fmt.Errorf("Помилка рендерингу SVG-вузла: %v", err)
	}

	// Зберігаємо серіалізований вміст у файл
	err := os.WriteFile(outputFilename, buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Помилка запису файлу %s: %v", outputFilename, err)
	}

	fmt.Printf("\n--> Успішно витягнуто та збережено SVG у файл: %s\n", outputFilename)
	return nil
}

// renderSVG рендерить SVG-вузол у правильному форматі XML з самозакриваючими тегами
func renderSVG(w io.Writer, n *html.Node) error {
	// Список SVG елементів, які повинні бути самозакриваючими
	selfClosingTags := map[string]bool{
		"circle": true, "ellipse": true, "line": true, "path": true,
		"polygon": true, "polyline": true, "rect": true, "use": true,
		"image": true, "stop": true, "animate": true, "animateMotion": true,
		"animateTransform": true, "set": true,
	}

	var render func(*html.Node, int) error
	render = func(n *html.Node, depth int) error {
		indent := ""
		for i := 0; i < depth; i++ {
			indent += "    "
		}

		switch n.Type {
		case html.ElementNode:
			// Відкриваючий тег
			fmt.Fprintf(w, "%s<%s", indent, n.Data)

			// Атрибути
			for _, attr := range n.Attr {
				fmt.Fprintf(w, " %s=\"%s\"", attr.Key, attr.Val)
			}

			// Перевірка чи є дочірні елементи
			hasChildren := n.FirstChild != nil

			// Якщо це самозакриваючий тег і немає дітей
			if selfClosingTags[n.Data] && !hasChildren {
				fmt.Fprintf(w, " />\n")
			} else if !hasChildren && n.Data != "svg" && n.Data != "g" && n.Data != "defs" && n.Data != "style" && n.Data != "text" {
				// Інші порожні елементи (крім контейнерів)
				fmt.Fprintf(w, " />\n")
			} else {
				fmt.Fprintf(w, ">")

				// Якщо це текстовий контейнер, не додаємо новий рядок
				if n.Data == "text" || n.Data == "style" {
					// Рендеримо дітей без відступів
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						if c.Type == html.TextNode {
							fmt.Fprint(w, c.Data)
						} else {
							render(c, 0)
						}
					}
					fmt.Fprintf(w, "</%s>\n", n.Data)
				} else {
					fmt.Fprint(w, "\n")
					// Рендеримо дочірні елементи
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						if err := render(c, depth+1); err != nil {
							return err
						}
					}
					fmt.Fprintf(w, "%s</%s>\n", indent, n.Data)
				}
			}

		case html.TextNode:
			// Пропускаємо порожні текстові вузли (пробіли між тегами)
			trimmed := bytes.TrimSpace([]byte(n.Data))
			if len(trimmed) > 0 {
				fmt.Fprintf(w, "%s%s\n", indent, n.Data)
			}

		case html.CommentNode:
			fmt.Fprintf(w, "%s<!-- %s -->\n", indent, n.Data)
		}

		return nil
	}

	return render(n, 0)
}

// findTag рекурсивно шукає вузол із заданим ім'ям тега і повертає його текстовий вміст.
func findTag(n *html.Node, tagName string) string {
	if n.Type == html.ElementNode && n.Data == tagName {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				return c.Data
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := findTag(c, tagName); result != "" {
			return result
		}
	}

	return ""
}

// traverse рекурсивно обходить усі вузли і викликає функцію 'f' для кожного з них.
// Якщо 'f' повертає true, обхід припиняється.
func traverse(n *html.Node, f func(*html.Node) bool) {
	if f(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		traverse(c, f)
	}
}

func main() {

	// 2. Створюємо вхідний файл, якщо він не існує
	if err := ensureFileExists(inputFilename); err != nil {
		fmt.Printf("Не вдалося створити або перевірити вхідний файл: %v\n", err)
		return
	}

	// 3. Відкриваємо файл для читання
	file, err := os.Open(inputFilename)
	if err != nil {
		fmt.Printf("Помилка відкриття файлу %s: %v\n", inputFilename, err)
		return
	}
	defer file.Close()

	fmt.Printf("Файл '%s' успішно відкрито.\n", inputFilename)

	// 4. Виконуємо парсинг, передаючи файл (io.Reader)
	doc, err := parseContent(file)
	if err != nil {
		return
	}

	// 5. Витягуємо SVG і зберігаємо його
	if err := extractAndSaveSVG(doc, targetSVGFilename); err != nil {
		fmt.Printf("Помилка витягнення SVG: %v\n", err)
		return
	}

	// 6. Конвертуємо SVG в PNG
	if err := convertSVGToPNG(targetSVGFilename, targetPNGFilename, 2450, 830); err != nil {
		fmt.Printf("Помилка конвертації SVG в PNG: %v\n", err)
		return
	}
}

// convertSVGToPNG конвертує SVG файл у PNG з заданими розмірами
// Використовує rsvg-convert для кращої підтримки всіх SVG можливостей
func convertSVGToPNG(svgFilename, pngFilename string, width, height int) error {
	// Перевіряємо чи встановлений rsvg-convert
	_, err := exec.LookPath("rsvg-convert")
	if err != nil {
		// Якщо rsvg-convert не встановлено, пробуємо використати oksvg
		return convertSVGToPNGWithOksvg(svgFilename, pngFilename, width, height)
	}

	// Для PNG створюємо SVG БЕЗ дзеркального відображення, читаючи з HTML
	// Читаємо оригінальний HTML файл
	htmlFile := inputFilename
	htmlData, err := os.ReadFile(htmlFile)
	if err != nil {
		return fmt.Errorf("помилка читання HTML: %v", err)
	}

	// Парсимо HTML і витягуємо SVG
	doc, err := html.Parse(bytes.NewReader(htmlData))
	if err != nil {
		return fmt.Errorf("помилка парсингу HTML: %v", err)
	}

	// Знаходимо SVG вузол
	var svgNode *html.Node
	var findSVG func(*html.Node)
	findSVG = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "svg" {
			svgNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if svgNode != nil {
				return
			}
			findSVG(c)
		}
	}
	findSVG(doc)

	if svgNode == nil {
		return fmt.Errorf("не знайдено SVG у HTML")
	}

	// Серіалізуємо SVG без трансформацій
	var buf bytes.Buffer
	if err := renderSVG(&buf, svgNode); err != nil {
		return fmt.Errorf("помилка рендерингу SVG: %v", err)
	}

	// Видаляємо дзеркальні трансформації для нормального PNG
	cleanSVG := replaceTransform(buf.String())

	// Зберігаємо у тимчасовий файл
	tempSVG := "temp_for_png.svg"
	if err := os.WriteFile(tempSVG, []byte(cleanSVG), 0644); err != nil {
		return fmt.Errorf("помилка створення тимчасового SVG: %v", err)
	}
	defer os.Remove(tempSVG)

	// Використовуємо rsvg-convert з білим фоном
	cmd := exec.Command("rsvg-convert",
		"-w", fmt.Sprintf("%d", width),
		"-b", "white",
		"--keep-aspect-ratio",
		"-o", pngFilename,
		tempSVG)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("помилка конвертації через rsvg-convert: %v\nВивід: %s", err, string(output))
	}

	fmt.Printf("\n--> Успішно створено PNG файл через rsvg-convert: %s (розмір: %dx%d)\n", pngFilename, width, height)
	return nil
}

// flipPNGFile читає PNG, дзеркально відображає його та зберігає назад
func flipPNGFile(filename string) error {
	// Читаємо PNG
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	img, err := png.Decode(file)
	file.Close()
	if err != nil {
		return err
	}

	// Конвертуємо в RGBA якщо потрібно
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}

	// Дзеркально відображаємо
	flipped := flipHorizontal(rgba)

	// Зберігаємо назад
	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return png.Encode(outFile, flipped)
}

// convertSVGToPNGWithOksvg - запасний метод конвертації через oksvg (обмежена підтримка)
func convertSVGToPNGWithOksvg(svgFilename, pngFilename string, width, height int) error {
	fmt.Println("УВАГА: rsvg-convert не встановлено. Використовується oksvg (обмежена підтримка тексту)")
	fmt.Println("Для кращої якості встановіть: sudo apt-get install librsvg2-bin")

	// Читаємо SVG файл
	svgData, err := os.ReadFile(svgFilename)
	if err != nil {
		return fmt.Errorf("помилка читання SVG файлу: %v", err)
	}

	// Видаляємо трансформації, які oksvg не підтримує
	svgString := string(svgData)
	svgString = replaceTransform(svgString)

	// Перевіряємо, чи потрібно дзеркально відобразити
	needsFlip := bytes.Contains(svgData, []byte(`transform="scale(-1, 1)"`))

	// Парсимо SVG
	icon, err := oksvg.ReadIconStream(bytes.NewReader([]byte(svgString)))
	if err != nil {
		return fmt.Errorf("помилка парсингу SVG: %v", err)
	}

	// Встановлюємо розміри
	icon.SetTarget(0, 0, float64(width), float64(height))

	// Створюємо зображення з білим фоном
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Заповнюємо білим фоном
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, image.White)
		}
	}

	// Рендеримо SVG
	scanner := rasterx.NewScannerGV(width, height, img, img.Bounds())
	raster := rasterx.NewDasher(width, height, scanner)
	icon.Draw(raster, 1.0)

	// Дзеркально відображаємо якщо потрібно
	var finalImg image.Image = img
	if needsFlip {
		finalImg = flipHorizontal(img)
	}

	// Зберігаємо PNG
	outFile, err := os.Create(pngFilename)
	if err != nil {
		return fmt.Errorf("помилка створення PNG файлу: %v", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, finalImg); err != nil {
		return fmt.Errorf("помилка кодування PNG: %v", err)
	}

	fmt.Printf("\n--> Створено PNG файл через oksvg: %s (можливо без тексту)\n", pngFilename)
	return nil
}

// flipHorizontal дзеркально відображає зображення по горизонталі
func flipHorizontal(src *image.RGBA) *image.RGBA {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	flipped := image.NewRGBA(bounds)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			flipped.Set(width-1-x, y, src.At(x, y))
		}
	}

	return flipped
}

// replaceTransform видаляє атрибут transform зі SVG тега та CSS трансформації з текстових стилів
func replaceTransform(svgString string) string {
	result := svgString

	// 1. Видаляємо transform="scale(-1, 1)" з головного SVG тегу
	result = string(bytes.ReplaceAll([]byte(result), []byte(`transform="scale(-1, 1)"`), []byte("")))

	// 2. Видаляємо style="transform-origin: center;" з головного SVG тегу
	result = string(bytes.ReplaceAll([]byte(result), []byte(`style="transform-origin: center;"`), []byte("")))

	// 3. Видаляємо CSS трансформації з текстових стилів (незалежно від відступів)
	// Видаляємо цілі рядки з цими властивостями
	lines := bytes.Split([]byte(result), []byte("\n"))
	var cleanedLines [][]byte

	for _, line := range lines {
		lineStr := string(bytes.TrimSpace(line))
		// Пропускаємо рядки з CSS трансформаціями
		if lineStr == "transform: scale(-1, 1);" ||
			lineStr == "transform-box: fill-box;" ||
			lineStr == "transform-origin: center;" {
			continue
		}
		cleanedLines = append(cleanedLines, line)
	}

	return string(bytes.Join(cleanedLines, []byte("\n")))
}
