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
// UpdateJobStarted marks a job as processing, sets start time, and updates the struct instance
func UpdateJobStarted(job *Job) error {
    now := time.Now()
    job.Status = "processing"
    job.StartedAt = &now

    // This updates the row on disk based on the primary key, matching the RAM changes
    return DB.Model(job).Select("Status", "StartedAt").Updates(job).Error
}

// UpdateJobCompleted marks a job as completed with end time and updates the struct instance
func UpdateJobCompleted(job *Job) error {
    now := time.Now()
    job.Status = "completed"
    job.CompletedAt = &now

    return DB.Model(job).Select("Status", "CompletedAt").Updates(job).Error
}

// UpdateJobFailed marks a job as failed with error message and updates the struct instance
func UpdateJobFailed(job *Job, errorMsg string) error {
    now := time.Now()
    job.Status = "failed"
    job.CompletedAt = &now
    job.ErrorMessage = errorMsg

    return DB.Model(job).Select("Status", "CompletedAt", "ErrorMessage").Updates(job).Error
}

// GetJobByResultID retrieves a job by result ID
func GetJobByResultID(resultID uint) (*Job, error) {
	var job Job

	if err := DB.Where("result_id = ?", resultID).First(&job).Error; err != nil {
		return nil, fmt.Errorf("job not found for result: %w", err)
	}

	return &job, nil
}
