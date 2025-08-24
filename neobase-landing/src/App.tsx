import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import Navbar from './components/Navbar'
import HeroSection from './components/HeroSection'
import VideoSection from './components/VideoSection'
import SupportedTechnologiesSection from './components/SupportedTechnologiesSection'
import Footer from './components/Footer'
import CompactFeaturesSection from './components/CompactFeaturesSection'
import HowItWorksSection from './components/HowItWorksSection'
import ComparisonSection from './components/ComparisonSection'
import UseCasesSection from './components/UseCasesSection'
import FAQSection from './components/FAQSection'
import ContributeSection from './components/ContributeSection'
import EnterprisePage from './components/EnterprisePage'
import PrivacyPolicy from './components/PrivacyPolicy'
import TermsOfService from './components/TermsOfService'
import ScrollToTop from './components/ScrollToTop'
import Clarity from '@microsoft/clarity';
import { initializeApp } from "firebase/app";
import { getAnalytics, logEvent } from "firebase/analytics";
import { useEffect, useState } from 'react';

function App() {
  const [stars, setStars] = useState(0);
  const [forks, setForks] = useState(0);
  useEffect(() => {
    initializeAnalytics();
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



  return (
    <Router>
      <ScrollToTop />
      <Routes>
        <Route path="/" element={
          <div className="min-h-screen bg-[#FFDB58]/10 overflow-hidden">
            <Navbar forks={forks}/>
            <main className="flex flex-col space-y-8 md:space-y-0">
              <HeroSection />
              <VideoSection />
              <UseCasesSection />
              <SupportedTechnologiesSection />
              <CompactFeaturesSection stars={stars}/>
              <HowItWorksSection />
              <ComparisonSection />
              <FAQSection />
              <ContributeSection />
            </main>
            <Footer />
          </div>
        } />
        <Route path="/enterprise" element={<EnterprisePage />} />
        <Route path="/privacy" element={<PrivacyPolicy />} />
        <Route path="/terms" element={<TermsOfService />} />
      </Routes>
    </Router>
  )
}

function initializeAnalytics() {
  console.log('Initializing analytics');
  // Initialize Microsoft Clarity
  if (import.meta.env.VITE_CLARITY_PROJECT_ID) {
    Clarity.init(import.meta.env.VITE_CLARITY_PROJECT_ID);
    console.log('Clarity initialized');
  }

  // Initialize Firebase Analytics
  const firebaseConfig = {
    apiKey: import.meta.env.VITE_FIREBASE_API_KEY,
    authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN,
    projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID,
    storageBucket: import.meta.env.VITE_FIREBASE_STORAGE_BUCKET,
    messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID,
    appId: import.meta.env.VITE_FIREBASE_APP_ID,
    measurementId: import.meta.env.VITE_FIREBASE_MEASUREMENT_ID
  };

  // Only initialize Firebase if the required environment variables are set
  if (import.meta.env.VITE_FIREBASE_API_KEY && import.meta.env.VITE_FIREBASE_MEASUREMENT_ID) {
    // Initialize Firebase
    const app = initializeApp(firebaseConfig);
    const analytics = getAnalytics(app);
    
    console.log('Firebase initialized');
    // Log page view event
    logEvent(analytics, 'page_view');
    
    // Track custom events for different sections
    trackSectionViews(analytics);
  }
}

// Function to track when users view different sections
function trackSectionViews(analytics: any) {
  // Use Intersection Observer to track when sections come into view
  const sections = [
    { id: 'hero', name: 'hero_section_view' },
    { id: 'video', name: 'video_section_view' },
    { id: 'technologies', name: 'technologies_section_view' },
    { id: 'features', name: 'features_section_view' },
    { id: 'how-it-works', name: 'how_it_works_section_view' },
    { id: 'comparison', name: 'comparison_section_view' },
    { id: 'use-cases', name: 'use_cases_section_view' },
    { id: 'faq', name: 'faq_section_view' },
    { id: 'contribute', name: 'contribute_section_view' }
  ];

  sections.forEach(section => {
    const element = document.getElementById(section.id);
    if (element) {
      const observer = new IntersectionObserver(
        (entries) => {
          entries.forEach(entry => {
            if (entry.isIntersecting) {
              logEvent(analytics, section.name);
              observer.unobserve(entry.target); // Only track once
            }
          });
        },
        { threshold: 0.5 } // Fire when 50% of the element is visible
      );
      observer.observe(element);
    }
  });
}

export default App
