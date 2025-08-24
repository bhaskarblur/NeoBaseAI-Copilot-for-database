import { useState, useEffect } from 'react';

export function useGitHubStats() {
  const [stars, setStars] = useState(0);
  const [forks, setForks] = useState(0);

  useEffect(() => {
    fetchStats();
  }, []);

  function fetchStats() {
    // GitHub API endpoint for the repository
    const repoUrl = "https://api.github.com/repos/bhaskarblur/neobase-ai-dba";
    
    // Fetch repository data from GitHub API
    fetch(repoUrl)
      .then(response => {
        if (!response.ok) {
          throw new Error(`GitHub API request failed: ${response.status}`);
        }
        return response.json();
      })
      .then(data => {
        // Update state with stars and forks count
        setStars(data.stargazers_count);
        setForks(data.forks_count);
        console.log(`Fetched GitHub stats: ${data.stargazers_count} stars, ${data.forks_count} forks`);
      })
      .catch(error => {
        console.error("Error fetching GitHub stats:", error);
        // Set fallback values in case of error
        setStars(14); // Current star count from search results
        setForks(4);  // Current fork count from search results
      });
  }

  return { stars, forks };
}