package api

import (
	"Inquisitor/db"
	"Inquisitor/printer"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// JobWorker processes background analysis jobs via a safe channel pipeline
type JobWorker struct {
	openaiKey string
	workers   int
	jobChan   chan *db.Job  // Channel to stream jobs instantly to waiting workers
	done      chan struct{} // Channel to orchestrate clean shutdowns
}

// NewJobWorker creates and initializes a new job worker with memory channels
func NewJobWorker(openaiKey string, numWorkers int) *JobWorker {
	return &JobWorker{
		openaiKey: openaiKey,
		workers:   numWorkers,
		jobChan:   make(chan *db.Job, numWorkers*2), // Buffered to handle sudden bursts
		done:      make(chan struct{}),
	}
}

// Start begins the coordinator loop and spawns execution workers
func (jw *JobWorker) Start() {
	// 1. Launch execution goroutines
	for i := 0; i < jw.workers; i++ {
		go jw.workerExecutor(i)
	}

	// 2. Launch exactly ONE coordinator thread to poll database for missed/old jobs
	go jw.coordinatorLoop()

	log.Printf("Started job processor orchestrator with %d execution workers", jw.workers)
}

// Submit pushes a job into the execution channel instantly from the HTTP handler
func (jw *JobWorker) Submit(job *db.Job) {
	select {
	case jw.jobChan <- job:
		// Job passed to an idle worker instantly! Bypasses the 2-second lag.
	default:
		// If the channel buffer is maxed out, log it.
		// The coordinator's database poll loop will grab it when queues clear.
		log.Printf("Worker queue buffer full, job %d deferred to DB polling backstop", job.ID)
	}
}

// coordinatorLoop ensures old, hanging, or missed jobs get processed eventually
func (jw *JobWorker) coordinatorLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Fetch jobs up to the concurrency capacity
			jobs, err := db.GetPendingJobs(jw.workers)
			if err != nil {
				log.Printf("Coordinator error querying pending jobs: %v", err)
				continue
			}

			// FIX: Loop by index 'i' to safely get the memory address of each job element
			for i := range jobs {
				// Atomically lock the job in the database inside our single thread
				if err := db.UpdateJobStarted(&jobs[i]); err != nil {
					continue
				}

				// FIX: Use the ampersand '&' on the slice item to pass it as a pointer (*db.Job)
				select {
				case jw.jobChan <- &jobs[i]:
				case <-jw.done:
					return
				}
			}

		case <-jw.done:
			return
		}
	}
}

// workerExecutor represents an isolated worker thread consuming jobs from the channel
func (jw *JobWorker) workerExecutor(workerID int) {
	for {
		select {
		case job := <-jw.jobChan:
			// Safety catch if the application started cutting off threads mid-queue
			if jw.isShuttingDown() {
				return
			}

			log.Printf("[Worker %d] Picked up Job ID: %d instantly", workerID, job.ID)
			if err := jw.executeJob(job); err != nil {
				log.Printf("[Worker %d] Failed processing job %d: %v", workerID, job.ID, err)
			}
		case <-jw.done:
			return
		}
	}
}

// executeJob contains strict error-checking boundaries for OpenAI execution
func (jw *JobWorker) executeJob(job *db.Job) error {
	result, err := db.GetResultByID(job.ResultID)
	if err != nil {
		_ = db.UpdateJobFailed(job, "internal db error: "+err.Error())
		return err
	}

	var questions []string
	if err := json.Unmarshal(result.QuestionsExtracted, &questions); err != nil {
		_ = db.UpdateJobFailed(job, "failed to parse extracted questions JSON")
		return err
	}

	// Safety Check: If there's nothing to process, skip OpenAI cleanly
	if len(questions) == 0 {
		_ = db.UpdateJobCompleted(job)
		return nil
	}

	// Verify OpenAI configurations are present
	if jw.openaiKey == "" {
		_ = db.UpdateJobFailed(job, "system error: OpenAI API key configuration missing")
		return fmt.Errorf("openai key missing")
	}

	// 1. CRITICAL FIX: Treat OpenAI failures as hard processing errors!
	var responses []string
	batchAnalysis, err := printer.BatchAnalyzeQuestionsWithGPT(jw.openaiKey, questions)
	if err != nil {
		log.Printf("[CRITICAL] OpenAI Batch Analysis failed for Job %d: %v", job.ID, err)
		
		// Revert job state to 'pending' or mark as 'failed' with reason so it can be re-evaluated
		_ = db.UpdateJobFailed(job, "OpenAI API timeout or rate-limit: "+err.Error())
		
		// Return the error so the worker loop knows this job actually FAILED
		return fmt.Errorf("openai batch failed: %w", err)
	}
	responses = append(responses, batchAnalysis)

	// 2. Treat individual analysis failures as hard errors too
	individualResponses, err := printer.AnalyzeQuestionsWithGPT(jw.openaiKey, questions)
	if err != nil {
		log.Printf("[CRITICAL] OpenAI Individual Analysis failed for Job %d: %v", job.ID, err)
		_ = db.UpdateJobFailed(job, "OpenAI individual processing failed: "+err.Error())
		return fmt.Errorf("openai individual failed: %w", err)
	}
	responses = append(responses, individualResponses...)

	responsesJSON, err := json.Marshal(responses)
	if err != nil {
		_ = db.UpdateJobFailed(job, "payload serialization crash")
		return err
	}

	// Save back to the database
	if err := db.DB.Model(&db.Result{}).Where("id = ?", job.ResultID).Update("ai_responses", responsesJSON).Error; err != nil {
		log.Printf("Failed to write AI responses to DB for job %d: %v", job.ID, err)
		_ = db.UpdateJobFailed(job, "database write failure")
		return err
	}

	// ONLY MARK AS COMPLETED IF EVERYTHING ABOVE PASSED WITH ZERO ERRORS
	return db.UpdateJobCompleted(job)
}

// Stop safely terminates processing and kills tickers without leakage
func (jw *JobWorker) Stop() {
	close(jw.done)
}

func (jw *JobWorker) isShuttingDown() bool {
	select {
	case <-jw.done:
		return true
	default:
		return false
	}
}