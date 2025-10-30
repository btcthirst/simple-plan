package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

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
	inputFilename = "plan2.html"
	// 1. Визначаємо ім'я вихідного файлу
	targetSVGFilename = "2.svg"
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

	// 1. Серіалізуємо SVG-вузол
	// html.Render приймає io.Writer і вузол, і записує повний HTML/XML цього вузла.
	var buf bytes.Buffer
	if err := html.Render(&buf, svgNode); err != nil {
		return fmt.Errorf("Помилка рендерингу SVG-вузла: %v", err)
	}

	// 2. Зберігаємо серіалізований вміст у файл
	err := os.WriteFile(outputFilename, buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Помилка запису файлу %s: %v", outputFilename, err)
	}

	fmt.Printf("\n--> Успішно витягнуто та збережено SVG у файл: %s\n", outputFilename)
	return nil
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
	}
}
