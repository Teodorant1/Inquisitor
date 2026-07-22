package printer

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

type PDFConfig struct {
	PageWidth            float64  `json:"page_width,omitempty"`
	PageHeight           float64  `json:"page_height,omitempty"`
	MarginLeft           float64  `json:"margin_left,omitempty"`
	MarginRight          float64  `json:"margin_right,omitempty"`
	MarginTop            float64  `json:"margin_top,omitempty"`
	MarginBottom         float64  `json:"margin_bottom,omitempty"`
	WatermarkTexts       []string `json:"watermark_texts,omitempty"`
	WatermarkAngle       float64  `json:"watermark_angle,omitempty"`
	WatermarkOpacity     float64  `json:"watermark_opacity,omitempty"`
	WatermarkFontSize    float64  `json:"watermark_font_size,omitempty"`
	WatermarkSeparator   string   `json:"watermark_separator,omitempty"`
	WarningTitle         string   `json:"warning_title,omitempty"`
	WarningColor         string   `json:"warning_color,omitempty"`
	WarningFontSize      float64  `json:"warning_font_size,omitempty"`
	WarningLineHeight    float64  `json:"warning_line_height,omitempty"`
	QuestionsTitle       string   `json:"questions_title,omitempty"`
	QuestionFontSize     float64  `json:"question_font_size,omitempty"`
	QuestionColor        string   `json:"question_color,omitempty"`
	QuestionLineHeight   float64  `json:"question_line_height,omitempty"`
	SeparatorLineColor   string   `json:"separator_line_color,omitempty"`
	AnswerLineHeight     float64  `json:"answer_line_height,omitempty"`
	FooterText           string   `json:"footer_text,omitempty"`
	FooterColor          string   `json:"footer_color,omitempty"`
	FooterFontSize       float64  `json:"footer_font_size,omitempty"`
	FontName             string   `json:"font_name,omitempty"`
	FontFilePath         string   `json:"font_file_path,omitempty"`
	WatermarkStepX       float64  `json:"watermark_step_x,omitempty"`
	WatermarkStepY       float64  `json:"watermark_step_y,omitempty"`
	AutoPageBreakBuffer  float64  `json:"auto_page_break_buffer,omitempty"`
}

func (c *PDFConfig) ApplyDefaults(useDefault bool) {
	// If useDefault is false, DO NOT set any default values.
	// Only protect against zero-value canvas dimensions to avoid gofpdf panic.
	if !useDefault {
		if c.PageWidth <= 0 { c.PageWidth = 210.0 }
		if c.PageHeight <= 0 { c.PageHeight = 297.0 }
		if c.FontName == "" { c.FontName = "Arial" }
		if c.WatermarkSeparator == "" { c.WatermarkSeparator = "                |                " }
		return
	}

	fmt.Println("[Info]: Applying default PDF configuration values for A4 layout and security watermarks.")

	c.WatermarkTexts = []string{"UNAUTHORIZED AI USE PROHIBITED", "ACADEMIC INTEGRITY MONITOR"}
	c.WatermarkAngle = -45.0
	c.WatermarkOpacity = 0.15
	c.WatermarkFontSize = 8.0
	c.WatermarkSeparator = "                |                "
	
	c.WarningTitle = "VIOLATION OF ACADEMIC INTEGRITY - AI USE PROHIBITED AND MONITORED"
	c.WarningColor = "#FF0000"
	c.WarningFontSize = 11.0
	c.WarningLineHeight = 6.0

	c.QuestionsTitle = "Exam Questions"
	c.QuestionFontSize = 10.0
	c.QuestionColor = "#000000"
	c.QuestionLineHeight = 5.0
	c.SeparatorLineColor = "#B4B4B4"
	c.AnswerLineHeight = 8.0

	c.FooterText = "SECURED BY INQUISITOR SYSTEM WORKFLOWS"
	c.FooterColor = "#640000"
	c.FooterFontSize = 7.0

	c.FontName = "NotoSans"
	c.FontFilePath = "./NotoSans-Regular.ttf"
	c.WatermarkStepX = 78.0
	c.WatermarkStepY = 34.0

	c.PageWidth = 210.0
	c.PageHeight = 297.0
	c.MarginLeft = 10.0
	c.MarginRight = 10.0
	c.MarginTop = 15.0
	c.MarginBottom = 15.0
	c.AutoPageBreakBuffer = 10.0
}
func HexToRGB(hexStr string) (int, int, int) {
	hexStr = strings.TrimPrefix(hexStr, "#")
	if len(hexStr) != 6 {
		return 0, 0, 0
	}
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return 0, 0, 0
	}
	return int(bytes[0]), int(bytes[1]), int(bytes[2])
}

func GenerateDynamicProtectedPDF(outputPDF string, questions []string, cfg PDFConfig, useDefault bool) error {
	cfg.ApplyDefaults(useDefault)

	// 1. Fully custom page size initialization
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr: "mm",
		Size:    gofpdf.SizeType{Wd: cfg.PageWidth, Ht: cfg.PageHeight},
	})

	// 2. Load primary custom font or fallback safely
	if cfg.FontFilePath != "" {
		if _, err := os.Stat(cfg.FontFilePath); err == nil {
			pdf.AddUTF8Font(cfg.FontName, "", cfg.FontFilePath)
		} else {
			fmt.Printf("[Warning]: Font file %s missing. Falling back to Arial.\n", cfg.FontFilePath)
			cfg.FontName = "Arial"
		}
	} else {
		cfg.FontName = "Arial"
	}

	// 3. Set Header Func: Executes on EVERY page creation
	pdf.SetHeaderFunc(func() {
		pageW, pageH := pdf.GetPageSize()

		// Background Watermark Grid
		if len(cfg.WatermarkTexts) > 0 {
			pdf.SetFont(cfg.FontName, "", cfg.WatermarkFontSize)
			pdf.SetTextColor(180, 180, 180)
			pdf.SetAlpha(cfg.WatermarkOpacity, "Normal")

			watermarkStr := strings.Join(cfg.WatermarkTexts, cfg.WatermarkSeparator)

			// Calculate canvas bleed based on angle & page size
			for y := -pageH * 0.2; y < pageH*1.5; y += cfg.WatermarkStepY {
				for x := -pageW * 0.5; x < pageW*1.5; x += cfg.WatermarkStepX {
					pdf.TransformBegin()
					pdf.TransformRotate(cfg.WatermarkAngle, x, y)
					pdf.Text(x, y, watermarkStr)
					pdf.TransformEnd()
				}
			}
			pdf.SetAlpha(1.0, "Normal")
		}

		// Warning Header Text
		if cfg.WarningTitle != "" {
			wr, wg, wb := HexToRGB(cfg.WarningColor)
			pdf.SetTextColor(wr, wg, wb)
			pdf.SetFont(cfg.FontName, "", cfg.WarningFontSize)

			pdf.SetXY(cfg.MarginLeft, cfg.MarginTop)
			pdf.MultiCell(0, cfg.WarningLineHeight, cfg.WarningTitle, "", "C", false)
		}
	})

	// 4. Set Footer Func: Executes on EVERY page creation
	pdf.SetFooterFunc(func() {
		if cfg.FooterText != "" {
			fr, fg, fb := HexToRGB(cfg.FooterColor)
			pdf.SetTextColor(fr, fg, fb)
			pdf.SetFont(cfg.FontName, "", cfg.FooterFontSize)
			pdf.SetXY(cfg.MarginLeft, cfg.PageHeight-cfg.MarginBottom)
			pdf.CellFormat(0, 5, cfg.FooterText, "", 0, "C", false, 0, "")
		}
	})

	// 5. Margin and Pagination Configuration
	pdf.SetMargins(cfg.MarginLeft, cfg.MarginTop, cfg.MarginRight)
	pdf.SetAutoPageBreak(true, cfg.MarginBottom+cfg.AutoPageBreakBuffer)

	// Create Page 1
	pdf.AddPage()

	// 6. Dynamic Content Placement Loop
	qr, qg, qb := HexToRGB(cfg.QuestionColor)
	pdf.SetTextColor(qr, qg, qb)
	pdf.SetFont(cfg.FontName, "", cfg.QuestionFontSize)

	lr, lg, lb := HexToRGB(cfg.SeparatorLineColor)

	// Calculate initial Y position dynamically below the header text
	initialY := pdf.GetY() + 4.0
	if initialY < cfg.MarginTop+10.0 {
		initialY = cfg.MarginTop + 10.0
	}
	pdf.SetY(initialY)

	for _, question := range questions {
		// Output Question
		pdf.MultiCell(0, cfg.QuestionLineHeight, question, "", "L", false)
		pdf.Ln(2)

		// Draw Separator Line
		lineY := pdf.GetY()
		pdf.SetDrawColor(lr, lg, lb)
		pdf.Line(cfg.MarginLeft, lineY, cfg.PageWidth-cfg.MarginRight, lineY)

		// Advance position for answer space / next item
		pdf.SetY(lineY + cfg.AnswerLineHeight)
	}

	if pdf.Err() {
		return fmt.Errorf("gofpdf internal error stream: %v", pdf.Error())
	}

	return pdf.OutputFileAndClose(outputPDF)
}

func ExtractUploadedPDF(filename string) ([]string, error) {
	_, lerr := exec.LookPath("pdftotext")
	if lerr != nil {
		return nil, fmt.Errorf("system binary 'pdftotext' is missing from the host path. Please install poppler-utils")
	}

	cmd := exec.Command("pdftotext", "-enc", "UTF-8", filename, "-")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext execution failed: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	var cleaned []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			cleaned = append(cleaned, strings.TrimSpace(line))
		}
	}
	return cleaned, nil
}