package infrastructure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

type GeminiClient struct {
	apiKey string
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient() *GeminiClient {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		panic("GEMINI_API_KEY environment variable not set")
	}
	return &GeminiClient{apiKey: apiKey}
}

// ExtractTextFromFile extracts text from files including PDF
func (g *GeminiClient) ExtractTextFromFile(file multipart.File, filename string) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	fileExtension := strings.ToLower(filename[strings.LastIndex(filename, ".")+1:])

	switch fileExtension {
	case "txt":
		// Handle text files directly
		return string(data), nil
	case "pdf":
		// Handle PDF files with multiple fallback methods
		return g.extractTextFromPDFWithFallback(data)
	default:
		// For other file types, try to extract as much text as possible
		if len(data) > 10000 {
			data = data[:10000] // Truncate very long content
		}
		return string(data), nil
	}
}

// extractTextFromPDFWithFallback tries multiple methods to extract text from PDF
func (g *GeminiClient) extractTextFromPDFWithFallback(data []byte) (string, error) {
	// Method 1: Try standard PDF text extraction
	text, err := g.extractTextFromPDF(data)
	if err == nil && text != "" {
		return text, nil
	}

	// Method 2: Try Gemini API for PDF text extraction
	fmt.Println("Standard PDF extraction failed, trying Gemini API...")
	geminiText, err := g.extractTextFromPDFWithGemini(data)
	if err == nil && geminiText != "" {
		return geminiText, nil
	}

	// Method 3: Return raw data as string (fallback)
	fmt.Println("All extraction methods failed, returning raw data...")
	if len(data) > 5000 {
		return string(data[:5000]), nil
	}
	return string(data), nil
}

// extractTextFromPDF extracts text from PDF files using unipdf
func (g *GeminiClient) extractTextFromPDF(data []byte) (string, error) {
	// Create a PDF reader from byte data
	pdfReader, err := model.NewPdfReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to read PDF: %w", err)
	}

	// Get number of pages
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return "", fmt.Errorf("failed to get page count: %w", err)
	}

	if numPages == 0 {
		return "", fmt.Errorf("PDF has no pages")
	}

	var textBuilder strings.Builder
	extractedAnyText := false

	// Extract text from each page
	for i := 1; i <= numPages; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			fmt.Printf("Error getting page %d: %v\n", i, err)
			continue // Skip pages with errors
		}

		ex, err := extractor.New(page)
		if err != nil {
			fmt.Printf("Error creating extractor for page %d: %v\n", i, err)
			continue
		}

		pageText, err := ex.ExtractText()
		if err != nil {
			fmt.Printf("Error extracting text from page %d: %v\n", i, err)
			continue
		}

		if pageText != "" {
			extractedAnyText = true
			textBuilder.WriteString(fmt.Sprintf("--- Page %d ---\n", i))
			textBuilder.WriteString(pageText)
			textBuilder.WriteString("\n\n")
		} else {
			fmt.Printf("No text found on page %d\n", i)
		}
	}

	result := strings.TrimSpace(textBuilder.String())
	if !extractedAnyText {
		return "", fmt.Errorf("no text could be extracted from any page of the PDF")
	}

	fmt.Printf("Successfully extracted text from PDF (%d characters)\n", len(result))
	return result, nil
}

// extractTextFromPDFWithGemini uses Gemini API to extract text from PDF
func (g *GeminiClient) extractTextFromPDFWithGemini(data []byte) (string, error) {
	ctx := context.Background()

	// Encode PDF to base64
	encodedPDF := base64.StdEncoding.EncodeToString(data)

	prompt := `Extract ALL text content from this PDF document. Return ONLY the raw extracted text without any additional comments, formatting, or explanations. Include:

- Personal information (name, email, phone)
- Education history
- Work experience
- Skills and technologies
- Certifications
- Projects and achievements

Return the text exactly as it appears in the document.`

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": prompt,
					},
					{
						"inline_data": map[string]interface{}{
							"mime_type": "application/pdf",
							"data":      encodedPDF,
						},
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.1,
			"maxOutputTokens": 8192,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Try available models
	availableModels := []string{
		"gemini-2.0-flash-001",
		"gemini-2.0-flash",
		"gemini-2.5-flash",
		"gemini-flash-latest",
	}

	var lastError error
	for _, model := range availableModels {
		fmt.Printf("Trying Gemini model for PDF extraction: %s\n", model)

		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
			model, g.apiKey)

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			lastError = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 120 * time.Second} // Longer timeout for PDF processing
		resp, err := client.Do(req)
		if err != nil {
			lastError = err
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastError = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastError = fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
			continue
		}

		var apiResponse map[string]interface{}
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			lastError = err
			continue
		}

		text, err := extractTextFromResponse(apiResponse)
		if err != nil {
			lastError = err
			continue
		}

		if text != "" {
			fmt.Printf("Successfully extracted text from PDF using Gemini (%d characters)\n", len(text))
			return strings.TrimSpace(text), nil
		}
	}

	return "", fmt.Errorf("all Gemini models failed for PDF extraction: %w", lastError)
}

// Evaluate performs evaluation using Gemini API
func (g *GeminiClient) Evaluate(ctx context.Context, description string, rubric string, cv string, project string) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(
		`You are an evaluator. Use the following job description and rubric to evaluate:

Job Description:
%s

Rubric:
%s

CV Input:
%s

Project Input:
%s

Define at least these scoring parameters:
 CV Evaluation (Match Rate)
 Technical Skills Match (backend, databases, APIs, cloud, AI/LLM exposure).
 Experience Level (years, project complexity).
 Relevant Achievements (impact, scale).
 Cultural Fit (communication, learning attitude).
 Project Deliverable Evaluation
 
 Correctness (meets requirements: prompt design, chaining, RAG, handling errors).
 Code Quality (clean, modular, testable).
 Resilience (handles failures, retries).
 Documentation (clear README, explanation of trade-offs).
 Creativity / Bonus (optional improvements like authentication, deployment, dashboards).
 Each parameter can be scored 1–5, then aggregated to final score

Return strict JSON with structure:
{
  "cv": {
    "match_rate": float,
    "feedback": string
  },
  "project": {
    "score": float,
    "feedback": string
  },
  "overall_summary": string
}

IMPORTANT: cv match_rate is between 0-1 and project score is between 1-10 and Return ONLY the raw JSON without any markdown formatting, code blocks, or additional text.`, description, rubric, cv, project)

	// Use the available models from your API
	availableModels := []string{
		"gemini-2.0-flash-001",
		"gemini-2.0-flash",
		"gemini-2.5-flash",
		"gemini-2.5-flash-preview-09-2025",
		"gemini-flash-latest",
	}

	var lastError error
	for _, model := range availableModels {
		fmt.Printf("Trying model for evaluation: %s\n", model)
		response, err := g.callGeminiWithModel(ctx, prompt, model)
		if err == nil {
			fmt.Printf("Success with model: %s\n", model)
			return response, nil
		}
		lastError = err
		fmt.Printf("Model %s failed: %v\n", model, err)
	}

	return nil, fmt.Errorf("all models failed: %w", lastError)
}

func (g *GeminiClient) callGeminiWithModel(ctx context.Context, prompt string, model string) (map[string]interface{}, error) {
	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": prompt,
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.1,
			"topP":        0.8,
			"topK":        40,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, g.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResponse map[string]interface{}
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	// Extract text from response
	text, err := extractTextFromResponse(apiResponse)
	if err != nil {
		return nil, err
	}

	cleanedContent := cleanJSONResponse(text)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedContent), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w\nResponse: %s", err, cleanedContent)
	}

	return result, nil
}

func extractTextFromResponse(apiResponse map[string]interface{}) (string, error) {
	candidates, ok := apiResponse["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	firstCandidate := candidates[0].(map[string]interface{})
	content, ok := firstCandidate["content"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return "", fmt.Errorf("no parts in content")
	}

	firstPart := parts[0].(map[string]interface{})
	text, ok := firstPart["text"].(string)
	if !ok {
		return "", fmt.Errorf("no text in part")
	}

	return text, nil
}

func cleanJSONResponse(content string) string {
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}

	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start != -1 && end != -1 && end > start {
		content = content[start : end+1]
	}

	return strings.TrimSpace(content)
}
