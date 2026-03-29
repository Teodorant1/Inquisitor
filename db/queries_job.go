package db

import (
	"fmt"
	"time"
)

// CreateJob creates a new job for async processing
func CreateJob(resultID uint) (*Job, error) {
	job := &Job{
		ResultID: resultID,
		Status:   "pending",
	}

	if err := DB.Create(job).Error; err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return job, nil
}

// GetJob retrieves a job by ID
func GetJob(jobID uint) (*Job, error) {
	var job Job

	if err := DB.First(&job, jobID).Error; err != nil {
		return nil, fmt.Errorf("job not found: %w", err)
	}

	return &job, nil
}

// GetPendingJobs retrieves all pending jobs
func GetPendingJobs(limit int) ([]Job, error) {
	var jobs []Job

	if err := DB.Where("status = ?", "pending").Limit(limit).Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch pending jobs: %w", err)
	}

	return jobs, nil
}

// UpdateJobStatus updates the status of a job
func UpdateJobStatus(jobID uint, status string) error {
	return DB.Model(&Job{}).Where("id = ?", jobID).Update("status", status).Error
}

// UpdateJobStarted marks a job as processing and sets start time
func UpdateJobStarted(jobID uint) error {
	now := time.Now()
	return DB.Model(&Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":     "processing",
		"started_at": &now,
	}).Error
}

// UpdateJobCompleted marks a job as completed with end time
func UpdateJobCompleted(jobID uint) error {
	now := time.Now()
	return DB.Model(&Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":       "completed",
		"completed_at": &now,
	}).Error
}

// UpdateJobFailed marks a job as failed with error message
func UpdateJobFailed(jobID uint, errorMsg string) error {
	now := time.Now()
	return DB.Model(&Job{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":         "failed",
		"completed_at":   &now,
		"error_message":  errorMsg,
	}).Error
}

// GetJobByResultID retrieves a job by result ID
func GetJobByResultID(resultID uint) (*Job, error) {
	var job Job

	if err := DB.Where("result_id = ?", resultID).First(&job).Error; err != nil {
		return nil, fmt.Errorf("job not found for result: %w", err)
	}

	return &job, nil
}
