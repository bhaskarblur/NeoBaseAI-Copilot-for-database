package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"neobase-ai/pkg/redis"
)

type GitHubService interface {
	GetStats(ctx context.Context) (map[string]interface{}, error)
}

type githubService struct {
	redisRepo redis.IRedisRepositories
}

type GitHubStarResponse struct {
	StargazersCount int `json:"stargazers_count"`
}

type GitHubForkResponse struct {
	ForkCount int `json:"forks_count"`
}

const (
	starCountKey     = "github:star_count"
	starCountTTL     = 7 * 24 * time.Hour
	githubRepoAPIURL = "https://api.github.com/repos/bhaskarblur/neobase-ai-dba"
	forkCountKey     = "github:fork_count"
)

func NewGitHubService(redisRepo redis.IRedisRepositories) GitHubService {
	return &githubService{
		redisRepo: redisRepo,
	}
}

func (s *githubService) GetStats(ctx context.Context) (map[string]interface{}, error) {
	// Try to get from cache first
	var starCount *int
	var forkCount *int
	cachedCount, err := s.redisRepo.Get(starCountKey, ctx)
	if err == nil && cachedCount != "" {
		if err := json.Unmarshal([]byte(cachedCount), &starCount); err == nil {
			log.Printf("GitHub star count fetched from cache: %d", starCount)
		}
	}

	cachedForkCount, err := s.redisRepo.Get(forkCountKey, ctx)
	if err == nil && cachedForkCount != "" {
		if err := json.Unmarshal([]byte(cachedForkCount), &forkCount); err == nil {
			log.Printf("GitHub fork count fetched from cache: %d", forkCount)
		}
	}

	if starCount != nil && forkCount != nil {
		return map[string]interface{}{
			"star_count": starCount,
			"fork_count": forkCount,
		}, nil
	}

	// Cache miss or error, fetch from GitHub
	count, err := s.fetchStarCount()
	if err != nil {
		log.Printf("Error fetching star count from GitHub: %v", err)
		return map[string]interface{}{}, err
	}

	fcount, err := s.fetchForkCount()
	if err != nil {
		log.Printf("Error fetching fork count from GitHub: %v", err)
		return map[string]interface{}{}, err
	}

	// Store in cache
	countBytes, _ := json.Marshal(count)
	if err := s.redisRepo.Set(starCountKey, countBytes, starCountTTL, ctx); err != nil {
		log.Printf("Error caching star count: %v", err)
	} else {
		log.Printf("GitHub star count cached successfully: %d", count)
	}
	forkCountBytes, _ := json.Marshal(fcount)
	if err := s.redisRepo.Set(forkCountKey, forkCountBytes, starCountTTL, ctx); err != nil {
		log.Printf("Error caching fork count: %v", err)
	} else {
		log.Printf("GitHub fork count cached successfully: %d", forkCount)
	}
	return map[string]interface{}{
		"star_count": count,
		"fork_count": fcount,
	}, nil
}

func (s *githubService) fetchStarCount() (int, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", githubRepoAPIURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	// Add User-Agent header to avoid GitHub API rate limiting
	req.Header.Set("User-Agent", "NeoBase-AI-DBA")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch from GitHub API: %v", err)
		return 0, fmt.Errorf("failed to fetch from GitHub API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("GitHub API returned status: %d", resp.StatusCode)
		return 0, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var repoData GitHubStarResponse
	if err := json.NewDecoder(resp.Body).Decode(&repoData); err != nil {
		return 0, fmt.Errorf("failed to decode GitHub response: %v", err)
	}

	log.Printf("GitHub star count fetched from API: %d", repoData.StargazersCount)
	return repoData.StargazersCount, nil
}

func (s *githubService) fetchForkCount() (int, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", githubRepoAPIURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("User-Agent", "NeoBase-AI-DBA")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch from GitHub API: %v", err)
		return 0, fmt.Errorf("failed to fetch from GitHub API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("GitHub API returned status: %d", resp.StatusCode)
		return 0, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var repoData GitHubForkResponse
	if err := json.NewDecoder(resp.Body).Decode(&repoData); err != nil {
		return 0, fmt.Errorf("failed to decode GitHub response: %v", err)
	}

	log.Printf("GitHub fork count fetched from API: %d", repoData.ForkCount)
	return repoData.ForkCount, nil
}
