package api

import (
	"encoding/json"
	"log"
	"time"

	"Inquisitor/db"
	"Inquisitor/printer"
)

// JobWorker processes background analysis jobs
type JobWorker struct {
	openaiKey string
	workers   int
}

// NewJobWorker creates a new job worker
func NewJobWorker(openaiKey string, numWorkers int) *JobWorker {
	return &JobWorker{
		openaiKey: openaiKey,
		workers:   numWorkers,
	}
}

// Start begins processing pending jobs
func (jw *JobWorker) Start() {
	for i := 0; i < jw.workers; i++ {
		go jw.processorLoop()
	}
	log.Printf("Started %d job processors", jw.workers)
}

// processorLoop continuously polls for pending jobs
func (jw *JobWorker) processorLoop() {
	ticker := time.NewTicker(2 * time.Second) // Poll every 2 seconds
	defer ticker.Stop()

	for range ticker.C {
		if err := jw.processPendingJob(); err != nil {
			log.Printf("Job processor error: %v", err)
		}
	}
}

// processPendingJob fetches and processes one pending job
func (jw *JobWorker) processPendingJob() error {
	// Get pending jobs
	jobs, err := db.GetPendingJobs(1)
	if err != nil {
		return err
	}

	if len(jobs) == 0 {
		return nil // No jobs to process
	}

	job := jobs[0]

	// Mark as processing
	if err := db.UpdateJobStarted(job.ID); err != nil {
		log.Printf("Failed to mark job %d as processing: %v", job.ID, err)
		return err
	}

	// Get the result
	result, err := db.GetResultByID(job.ResultID)
	if err != nil {
		log.Printf("Failed to fetch result for job %d: %v", job.ID, err)
		_ = db.UpdateJobFailed(job.ID, err.Error())
		return err
	}

	// Extract questions from JSON
	var questions []string
	if err := json.Unmarshal(result.QuestionsExtracted, &questions); err != nil {
		log.Printf("Failed to parse questions for job %d: %v", job.ID, err)
		_ = db.UpdateJobFailed(job.ID, "failed to parse questions")
		return err
	}

	// Process with GPT if PDF and OpenAI key is available
	var responses []string
	if result.InputType == "pdf" && jw.openaiKey != "" && len(questions) > 0 {
		// Batch analysis
		batchAnalysis, err := printer.BatchAnalyzeQuestionsWithGPT(jw.openaiKey, questions)
		if err != nil {
			log.Printf("Warning: batch analysis failed for job %d: %v", job.ID, err)
		} else {
			responses = append(responses, batchAnalysis)

			// Individual analyses
			individualResponses, err := printer.AnalyzeQuestionsWithGPT(jw.openaiKey, questions)
			if err != nil {
				log.Printf("Warning: individual analysis failed for job %d: %v", job.ID, err)
			} else {
				responses = append(responses, individualResponses...)
			}
		}
	}

	// Marshal responses to JSON
	responsesJSON, err := json.Marshal(responses)
	if err != nil {
		log.Printf("Failed to marshal responses for job %d: %v", job.ID, err)
		_ = db.UpdateJobFailed(job.ID, "failed to marshal responses")
		return err
	}

	// Update the result with responses
	if err := db.DB.Model(&db.Result{}).Where("id = ?", job.ResultID).Update("ai_responses", responsesJSON).Error; err != nil {
		log.Printf("Failed to update result for job %d: %v", job.ID, err)
		_ = db.UpdateJobFailed(job.ID, "failed to update result")
		return err
	}

	// Mark job as completed
	if err := db.UpdateJobCompleted(job.ID); err != nil {
		log.Printf("Failed to mark job %d as completed: %v", job.ID, err)
		return err
	}

	log.Printf("Job %d completed successfully for result %d", job.ID, job.ResultID)
	return nil
}
