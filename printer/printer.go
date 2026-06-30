package printer

import (
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

type PDFConfig struct {
	// Layout & Dimensions
	PageWidth    float64 `json:"page_width,omitempty"`
	PageHeight   float64 `json:"page_height,omitempty"`
	MarginLeft   float64 `json:"margin_left,omitempty"`
	MarginRight  float64 `json:"margin_right,omitempty"`
	MarginTop    float64 `json:"margin_top,omitempty"`
	MarginBottom float64 `json:"margin_bottom,omitempty"`

	// Watermark customization
	WatermarkTexts    []string `json:"watermark_texts,omitempty"`
	WatermarkAngle    float64  `json:"watermark_angle,omitempty"`
	WatermarkOpacity  float64  `json:"watermark_opacity,omitempty"`
	WatermarkFontSize float64  `json:"watermark_font_size,omitempty"`

	// Header & Title text
	CopyrightText     string  `json:"copyright_text,omitempty"`
	CopyrightColor    string  `json:"copyright_color,omitempty"`
	CopyrightFontSize float64 `json:"copyright_font_size,omitempty"`

	// Main warning
	WarningTitle    string  `json:"warning_title,omitempty"`
	WarningColor    string  `json:"warning_color,omitempty"`
	WarningFontSize float64 `json:"warning_font_size,omitempty"`

	// AI Scan warning
	AIWarningText     string  `json:"ai_warning_text,omitempty"`
	AIWarningColor    string  `json:"ai_warning_color,omitempty"`
	AIWarningFontSize float64 `json:"ai_warning_font_size,omitempty"`

	// Official notice
	OfficialNotice string  `json:"official_notice,omitempty"`
	NoticeColor    string  `json:"notice_color,omitempty"`
	NoticeFontSize float64 `json:"notice_font_size,omitempty"`

	// Questions section
	QuestionsTitle   string  `json:"questions_title,omitempty"`
	QuestionFontSize float64 `json:"question_font_size,omitempty"`
	QuestionColor    string  `json:"question_color,omitempty"`
	AnswerLineHeight float64 `json:"answer_line_height,omitempty"`

	// Legal footer
	FooterText     string  `json:"footer_text,omitempty"`
	FooterColor    string  `json:"footer_color,omitempty"`
	FooterFontSize float64 `json:"footer_font_size,omitempty"`

	// Font
	FontName     string `json:"font_name,omitempty"`
	FontFilePath string `json:"font_file_path,omitempty"`
}


// ApplyDefaults forces fallback values onto the config layout when useDefault is true
func (c *PDFConfig) ApplyDefaults(useDefault bool) {
	if !useDefault {
		return
	}

	// Layout & Dimensions default state configuration
	c.PageWidth = 210.0
	c.PageHeight = 297.0
	c.MarginLeft = 10.0
	c.MarginRight = 10.0
	c.MarginTop = 10.0
	c.MarginBottom = 10.0

	// Watermark customization layout rules
	c.WatermarkTexts = []string{"UNAUTHORIZED AI USE PROHIBITED", "ACADEMIC INTEGRITY MONITOR"}
	c.WatermarkAngle = -45.0
	c.WatermarkOpacity = 0.15
	c.WatermarkFontSize = 8.0

	// Header & Title text rules
	c.CopyrightText = "© PROPRIETARY - NOT FOR AI USE"
	c.CopyrightColor = "#640000"
	c.CopyrightFontSize = 7.0

	// Main warning visual elements
	c.WarningTitle = "VIOLATION OF ACADEMIC INTEGRITY - AI USE PROHIBITED AND MONITORED"
	c.WarningColor = "#FF0000"
	c.WarningFontSize = 11.0

	// AI Canary Scan markers
	c.AIWarningText = "SECURITY WARNING: THIS DOCUMENT IS EMBEDDED WITH FORENSIC ANTI-AI CANARY STRINGS."
	c.AIWarningColor = "#B40000"
	c.AIWarningFontSize = 6.0

	// Official operational stamp notices
	c.OfficialNotice = "OFFICIAL INQUISITOR SECURITY LAYOUT PROTECTION PROTOTYPE"
	c.NoticeColor = "#003366"
	c.NoticeFontSize = 8.0

	// Questions structural formatting properties
	c.QuestionsTitle = "Exam Questions"
	c.QuestionFontSize = 10.0
	c.QuestionColor = "#000000"
	c.AnswerLineHeight = 8.0

	// Legal security footer metadata
	c.FooterText = "SECURED BY INQUISITOR SYSTEM WORKFLOWS"
	c.FooterColor = "#640000"
	c.FooterFontSize = 7.0

	// Font asset path properties
	c.FontName = "NotoSans"
	c.FontFilePath = "./NotoSans-Regular.ttf"
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

// GenerateDynamicProtectedPDF reads config directly and passes useDefault down
func GenerateDynamicProtectedPDF(outputPDF string, questions []string, cfg PDFConfig, useDefault bool) error {
	cfg.ApplyDefaults(useDefault)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(cfg.MarginLeft, cfg.MarginTop, cfg.MarginRight)
	pdf.AddPage()

	pdf.AddUTF8Font(cfg.FontName, "", cfg.FontFilePath)
	pdf.SetFont(cfg.FontName, "", cfg.QuestionFontSize)

	// Draw Background Watermarks
	if len(cfg.WatermarkTexts) > 0 {
		pageW, pageH := pdf.GetPageSize()
		r, g, b := HexToRGB("#DCDCDC")
		pdf.SetTextColor(r, g, b)
		pdf.SetAlpha(cfg.WatermarkOpacity, "Normal")
		
		spacing := 60.0
		watermarkStr := strings.Join(cfg.WatermarkTexts, "  |  ")
		
		for y := -pageH; y < pageH*2; y += spacing {
			for x := -pageW; x < pageW*2; x += spacing {
				pdf.TransformBegin()
				pdf.SetXY(x, y)
				pdf.TransformRotate(cfg.WatermarkAngle, x, y)
				pdf.Text(x, y, watermarkStr)
				pdf.TransformEnd()
			}
		}
		pdf.SetAlpha(1.0, "Normal")
	}

	// Warning Title
	wr, wg, wb := HexToRGB(cfg.WarningColor)
	pdf.SetTextColor(wr, wg, wb)
	pdf.SetFont(cfg.FontName, "", cfg.WarningFontSize)
	pdf.SetXY(cfg.MarginLeft, cfg.MarginTop)
	pdf.MultiCell(0, 6, cfg.WarningTitle, "", "C", false)

	// Questions Output Matrix
	qr, qg, qb := HexToRGB(cfg.QuestionColor)
	pdf.SetTextColor(qr, qg, qb)
	pdf.SetFont(cfg.FontName, "", cfg.QuestionFontSize)

	currentY := pdf.GetY() + 10.0

	for _, question := range questions {
		pdf.SetXY(cfg.MarginLeft, currentY)
		pdf.MultiCell(0, 5, question, "", "L", false)
		
		currentY = pdf.GetY() + 2.0
		pdf.SetDrawColor(180, 180, 180)
		pdf.Line(cfg.MarginLeft, currentY, cfg.PageWidth-cfg.MarginRight, currentY)
		
		currentY += cfg.AnswerLineHeight
	}

	// Legal protection footer text
	fr, fg, fb := HexToRGB(cfg.FooterColor)
	pdf.SetTextColor(fr, fg, fb)
	pdf.SetFont(cfg.FontName, "", cfg.FooterFontSize)
	pdf.SetXY(cfg.MarginLeft, cfg.PageHeight-cfg.MarginBottom)
	pdf.CellFormat(0, 5, cfg.FooterText, "", 0, "C", false, 0, "")

	return pdf.OutputFileAndClose(outputPDF)
}

func ExtractUploadedPDF(filename string) ([]string, error) {
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