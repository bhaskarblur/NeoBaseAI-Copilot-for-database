import axios from './axiosConfig';

const API_URL = import.meta.env.VITE_API_URL;

const githubService = {
  async getGithubStats(): Promise<{ star_count: number }> {
    try {
      const response = await axios.get(`${API_URL}/github/stats`);
      return response.data.data;
    } catch (error: any) {
      throw new Error(error.response?.data?.error || 'Failed to fetch star count');
    }
  },
};

export default githubService;
