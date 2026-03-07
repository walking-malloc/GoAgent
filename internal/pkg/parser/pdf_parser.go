package parser

import (
	"fmt"
	"os"

	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

// PDFParser PDF 文件解析器
type PDFParser struct{}

func (p *PDFParser) Parse(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader, err := model.NewPdfReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create PDF reader: %w", err)
	}

	numPages, err := reader.GetNumPages()
	if err != nil {
		return "", fmt.Errorf("failed to get page count: %w", err)
	}

	var content string
	for i := 1; i <= numPages; i++ {
		page, err := reader.GetPage(i)
		if err != nil {
			continue
		}

		ex, err := extractor.New(page)
		if err != nil {
			continue
		}

		text, err := ex.ExtractText()
		if err != nil {
			continue
		}

		if text != "" {
			content += text + "\n\n"
		}
	}

	return content, nil
}

func (p *PDFParser) Support(fileType string) bool {
	return fileType == "pdf"
}
