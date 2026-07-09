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
	PageWidth         float64  `json:"page_width,omitempty"`
	PageHeight        float64  `json:"page_height,omitempty"`
	MarginLeft        float64  `json:"margin_left,omitempty"`
	MarginRight       float64  `json:"margin_right,omitempty"`
	MarginTop         float64  `json:"margin_top,omitempty"`
	MarginBottom      float64  `json:"margin_bottom,omitempty"`
	WatermarkTexts    []string `json:"watermark_texts,omitempty"`
	WatermarkAngle    float64  `json:"watermark_angle,omitempty"`
	WatermarkOpacity  float64  `json:"watermark_opacity,omitempty"`
	WatermarkFontSize float64  `json:"watermark_font_size,omitempty"`
	CopyrightText     string   `json:"copyright_text,omitempty"`
	CopyrightColor    string   `json:"copyright_color,omitempty"`
	CopyrightFontSize float64  `json:"copyright_font_size,omitempty"`
	WarningTitle      string   `json:"warning_title,omitempty"`
	WarningColor      string   `json:"warning_color,omitempty"`
	WarningFontSize   float64  `json:"warning_font_size,omitempty"`
	AIWarningText     string   `json:"ai_warning_text,omitempty"`
	AIWarningColor    string   `json:"ai_warning_color,omitempty"`
	AIWarningFontSize float64  `json:"ai_warning_font_size,omitempty"`
	OfficialNotice    string   `json:"official_notice,omitempty"`
	NoticeColor       string   `json:"notice_color,omitempty"`
	NoticeFontSize    float64  `json:"notice_font_size,omitempty"`
	QuestionsTitle    string   `json:"questions_title,omitempty"`
	QuestionFontSize  float64  `json:"question_font_size,omitempty"`
	QuestionColor     string   `json:"question_color,omitempty"`
	AnswerLineHeight  float64  `json:"answer_line_height,omitempty"`
	FooterText        string   `json:"footer_text,omitempty"`
	FooterColor       string   `json:"footer_color,omitempty"`
	FooterFontSize    float64  `json:"footer_font_size,omitempty"`
	FontName          string   `json:"font_name,omitempty"`
	FontFilePath      string   `json:"font_file_path,omitempty"`
	WatermarkStepX    float64  `json:"watermark_step_x,omitempty"` // Added for dynamic grid width	
	WatermarkStepY    float64  `json:"watermark_step_y,omitempty"` // Added for dynamic grid height
}

func (c *PDFConfig) ApplyDefaults(useDefault bool) {
	if useDefault == false {
		return
	}
	fmt.Println("[Info]: Applying default PDF configuration values for A4 layout and security watermarks.")

	
	// c.PageWidth = 210.0
	// c.PageHeight = 297.0
	// c.MarginLeft = 10.0
	// c.MarginRight = 10.0
	// c.MarginTop = 10.0
	// c.MarginBottom = 10.0
	c.WatermarkTexts = []string{"UNAUTHORIZED AI USE PROHIBITED", "ACADEMIC INTEGRITY MONITOR"}
	c.WatermarkAngle = -45.0
	c.WatermarkOpacity = 0.15
	c.WatermarkFontSize = 8.0
	c.CopyrightText = "© PROPRIETARY - NOT FOR AI USE"
	c.CopyrightColor = "#640000"
	c.CopyrightFontSize = 7.0
	c.WarningTitle = "VIOLATION OF ACADEMIC INTEGRITY - AI USE PROHIBITED AND MONITORED"
	c.WarningColor = "#FF0000"
	// c.WarningFontSize = 11.0
	c.AIWarningText = "SECURITY WARNING: THIS DOCUMENT IS EMBEDDED WITH FORENSIC ANTI-AI CANARY STRINGS."
	c.AIWarningColor = "#B40000"
	c.AIWarningFontSize = 6.0
	c.OfficialNotice = "OFFICIAL INQUISITOR SECURITY LAYOUT PROTECTION PROTOTYPE"
	c.NoticeColor = "#003366"
	// c.NoticeFontSize = 8.0
	c.QuestionsTitle = "Exam Questions"
	// c.QuestionFontSize = 10.0
	c.QuestionColor = "#000000"
	c.AnswerLineHeight = 8.0
	c.FooterText = "SECURED BY INQUISITOR SYSTEM WORKFLOWS"
	c.FooterColor = "#640000"
	c.FooterFontSize = 7.0
	
	// Default custom configuration values
	c.FontName = "NotoSans"
	c.FontFilePath = "./NotoSans-Regular.ttf"
	c.WatermarkStepX = 88.0         // 40% of 220.0 (Brings elements closer horizontally)
	c.WatermarkStepY = 44.0

// Set explicit PostScript point units for true A4 dimensions
    c.PageWidth = 595.28  
    c.PageHeight = 841.89 
    c.MarginLeft = 28.35  
    c.MarginRight = 28.35 
    c.MarginTop = 28.35    
    c.MarginBottom = 28.35 
    
    // Scale up your baseline fonts relative to your new point grid layout
    c.QuestionFontSize = 12.0
    c.WarningFontSize = 14.0
    c.NoticeFontSize = 11.0

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

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(cfg.MarginLeft, cfg.MarginTop, cfg.MarginRight)
	pdf.AddPage()

	// AUTOMATIC FONT ESCAPE: Verify font on disk before loading
	if cfg.FontFilePath != "" {
		if _, err := os.Stat(cfg.FontFilePath); err == nil {
			pdf.AddUTF8Font(cfg.FontName, "", cfg.FontFilePath)
			pdf.SetFont(cfg.FontName, "", cfg.QuestionFontSize)
		} else {
			// Fail-safe default font configuration
			fmt.Printf("[Warning]: Font file %s missing. Falling back to Arial.\n", cfg.FontFilePath)
			cfg.FontName = "Arial"
			pdf.SetFont(cfg.FontName, "", cfg.QuestionFontSize)
		}
	} else {
		cfg.FontName = "Arial"
		pdf.SetFont(cfg.FontName, "", cfg.QuestionFontSize)
	}

// Draw Background Watermarks
	if len(cfg.WatermarkTexts) > 0 {
		pageW, pageH := pdf.GetPageSize()
		
		if cfg.FontName == "Arial" {
			pdf.SetFont("Arial", "", cfg.WatermarkFontSize)
		} else {
			pdf.SetFont(cfg.FontName, "", cfg.WatermarkFontSize)
		}
		
		pdf.SetTextColor(180, 180, 180)
		pdf.SetAlpha(cfg.WatermarkOpacity, "Normal")
		
		// 1. INCREASE HORIZONTAL GAP BETWEEN WORDS IN THE SAME LINE:
		// Changed from 6 spaces to 16 spaces to stretch it out horizontally
		watermarkStr := strings.Join(cfg.WatermarkTexts, "                |                ")
		
		// 2. INCREASE THE GRID STEPPING VALUES:
		// stepX: Controls horizontal distance between columns (Up from ~130)
		// stepY: Controls vertical distance between rows (Up from ~65)
		stepX := 90.0 
		stepY := 80.0  
		
		// Expanded bounds slightly to make sure margins are cleanly covered at wide steps
		for y := -20.0; y < pageH+150; y += stepY {
			for x := -100.0; x < pageW+150; x += stepX {
				pdf.TransformBegin()
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
	
	// Use standard fallback styling if custom fonts fail
	if cfg.FontName == "Arial" {
		pdf.SetFont("Arial", "B", cfg.WarningFontSize)
	} else {
		pdf.SetFont(cfg.FontName, "", cfg.WarningFontSize)
	}
	
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

	// Check for accumulated errors before closing
	if pdf.Err() {
		return fmt.Errorf("gofpdf internal error stream: %v", pdf.Error())
	}

	return pdf.OutputFileAndClose(outputPDF)
}

func ExtractUploadedPDF(filename string) ([]string, error) {
	// Verify that the pdftotext executable is available in the system PATH
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