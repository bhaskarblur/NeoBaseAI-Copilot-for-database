import { useState, useEffect } from 'react'
import { CheckCircle, Loader2, Boxes, Github } from 'lucide-react'
import StableFloatingBackground from './StableFloatingBackground'
import Footer from './Footer'
import { Link } from 'react-router-dom'
import axios from 'axios'
import { Toaster } from 'react-hot-toast'
import { toast } from './CustomToast'

const EnterprisePage = () => {
  const [email, setEmail] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [isSubmitted, setIsSubmitted] = useState(false)
  const [forks, setForks] = useState(0)

  // Set document title and smooth scroll to top on page load
  useEffect(() => {
    // Set document title for SEO
    document.title = 'NeoBase Enterprise - Copilot for Organizations Data sources'
    
    // Set meta description
    const metaDescription = document.querySelector('meta[name="description"]')
    if (metaDescription) {
      metaDescription.setAttribute('content', 'NeoBase Enterprise - Advanced data sources, database management platform with enterprise-grade security, on-premise deployment, and dedicated support for organizations.')
    }
    
    // Small delay to ensure page is rendered
    const scrollToTop = () => {
      window.scrollTo({
        top: 0,
        behavior: 'smooth'
      })
    }
    
    // Use setTimeout to ensure DOM is ready
    const timer = setTimeout(scrollToTop, 100)
    
    // Also try immediate scroll as fallback
    window.scrollTo(0, 0)
    
    // Cleanup: restore original title when leaving the page
    return () => {
      clearTimeout(timer)
      document.title = 'NeoBase - AI Copilot for Database'
      if (metaDescription) {
        metaDescription.setAttribute('content', 'NeoBase is an AI Copilot for Database that helps you query, analyze, and manage your databases with natural language. Connect to PostgreSQL, MySQL, and more.')
      }
    }
  }, [])

  // Fetch GitHub stats
  useEffect(() => {
    fetch("https://api.github.com/repos/bhaskarblur/neobase-ai-dba")
      .then(response => response.json())
      .then(data => {
        setForks(data.forks_count || 1)
      })
      .catch(() => setForks(4))
  }, [])


  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    if (!email) {
      toast.error('Please enter your email')
      return
    }

    setIsLoading(true)
    
    try {
      await axios.post(`${import.meta.env.VITE_NEOBASE_API_URL}/enterprise/waitlist`, {
        email
      })
      
      setIsSubmitted(true)
      toast.success('You\'ve been added to the waitlist!')
      setEmail('')
    } catch (error: any) {
      console.log(error)
      if(error.response?.status == 409) {
        toast.success('You\'re already on the waitlist!')
        return
      }
      toast.error(error.response?.data?.error || 'Something went wrong. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  const formatForkCount = (count: number): string => {
    if (count >= 1000) {
      return `${(count / 1000).toFixed(1)}k`
    }
    return count.toString()
  }

  return (
    <>
      <Toaster 
        position="bottom-center"
        toastOptions={{
          style: {
            maxWidth: '500px',
          }
        }}
      />
      <div className="min-h-screen bg-[#FFDB58]/10 overflow-hidden flex flex-col">
        {/* Custom Navbar for Enterprise page */}
        <nav className="py-4 px-6 md:px-8 lg:px-12 border-b-4 border-black bg-white">
          <div className="container mx-auto max-w-7xl">
            <div className="flex items-center justify-between">
              {/* Logo */}
              <Link to="/" className="flex items-center gap-2">
                <Boxes className="w-8 h-8" />
                <span className="text-2xl font-bold">NeoBase</span>
              </Link>

              {/* Right side with Community link and buttons */}
              <div className="flex items-center gap-6">
                <Link to="/" className="font-medium hover:text-gray-600 transition-colors hidden md:block">Home</Link>
                
                {/* Product Hunt Button */}
                <a href="https://www.producthunt.com/posts/neobase-2?embed=true&utm_source=badge-featured&utm_medium=badge&utm_souce=badge-neobase&#0045;2" target="_blank" className="hidden md:block">
                  <img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=936307&theme=light&t=1741073867985" alt="NeoBase - AI&#0032;powered&#0032;database&#0032;assistant | Product Hunt" style={{width: '220px', height: '48px'}} width="220" height="48" />
                </a>
                
                {/* Github Fork Button */}
                <a 
                  href="https://github.com/bhaskarblur/neobase-ai-dba" 
                  target="_blank" 
                  rel="noopener noreferrer"
                  className="neo-button hidden md:flex items-center gap-2 py-2 px-4 text-sm bg-black text-white"
                >
                  <Github className="w-4 h-4" />
                  <span>Fork Us</span>
                  <span className="bg-white/20 px-2 py-0.5 rounded-full text-xs font-mono">
                    {formatForkCount(forks || 1)}
                  </span>
                </a>
              </div>
            </div>
          </div>
        </nav>
        
        {/* Fundraising Banner */}
        <div className="bg-black text-white py-2 pt-1.5 px-6 md:px-8 lg:px-12 border-b-2 border-gray-700">
          <div className="container mx-auto max-w-7xl">
            <div className="flex items-center justify-center text-center">
              <p className="text-sm md:text-base font-medium leading-relaxed">
                🚀 We're raising <strong>Seed Funding</strong> round for developing Enterprise Version.{' '}
                <br className="sm:hidden" />
                To know more:{' '}
                <a 
                  href="https://drive.google.com/file/d/1uvjE-rMgkYA_LhkZCoQEruh_kYGj0pIa/view?usp=drive_link"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="ml-1 underline hover:text-yellow-400 transition-colors font-semibold"
                >
                  View our Pitch Deck
                </a>
              </p>
            </div>
          </div>
        </div>

        {/* Main Content */}
        <section className="py-12 sm:py-16 md:py-20 lg:py-28 relative">
          <StableFloatingBackground count={15} opacity={0.05} />
          
          <div className="container mx-auto px-4 sm:px-6 md:px-0 relative max-w-7xl">
            <div className="text-center max-w-4xl mx-auto">
              <div className="inline-block neo-border bg-[#FFDB58] px-3 sm:px-4 py-1.5 sm:py-2 font-bold text-sm sm:text-sm mb-6">
                #Next-Gen-DataOps
              </div>
              
              <h1 className="text-4xl sm:text-4xl md:text-5xl lg:text-6xl font-extrabold leading-tight mb-6">
                Enterprises -<span className="text-green-500"> On The Way</span>
              </h1>
              
              <p className="text-lg sm:text-lg md:text-xl text-gray-700 mb-8 max-w-3xl mx-auto">
                Unlock the full potential of NeoBase for your organization. 
                Advanced features, dedicated support, on-premise deployment, 
                and enterprise-grade security for teams that demand the best.
              </p>

              {!isSubmitted ? (
                <form onSubmit={handleSubmit} className="max-w-xl mx-auto">
                  <div className="flex flex-col sm:flex-row gap-4">
                    <input
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="Enter your work email"
                      className="neo-input flex-1"
                      disabled={isLoading}
                      required
                    />
                    <button
                      type="submit"
                      disabled={isLoading}
                      className="neo-button px-6 py-3 font-bold flex items-center justify-center gap-2 min-w-[180px]"
                    >
                      {isLoading ? (
                        <>
                          <Loader2 className="w-5 h-5 animate-spin" />
                          <span>Joining...</span>
                        </>
                      ) : (
                        <span>Join Waitlist</span>
                      )}
                    </button>
                  </div>
                  <p className="text-sm text-gray-600 mt-4">
                    Get notified about NeoBase Enterprise launch & other updates
                  </p>
                </form>
              ) : (
                <div className="max-w-lg mx-auto neo-border bg-green-50 p-6">
                  <CheckCircle className="w-12 h-12 text-green-600 mx-auto mb-4" />
                  <h3 className="text-xl font-bold mb-2">You're on the list!</h3>
                  <p className="text-gray-700">
                    We'll notify you as soon as NeoBase Enterprise is available. 
                    Check your email for more details.
                  </p>
                </div>
              )}
            </div>
          </div>
        </section>

        {/* Features Section - Full Width */}
        <section className="pb-12 sm:pb-16 md:pb-20 lg:pb-28 px-4 sm:px-6 md:px-8">
          <div className="max-w-7xl mx-auto">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 sm:gap-6">
              <div className="neo-border bg-white p-4 sm:p-6">
                <h3 className="font-bold text-base sm:text-lg mb-2">☝️ One Platform, Multiple Data Sources</h3>
                <p className="text-sm sm:text-base text-gray-600">Monitor activity across various sources at one place - NeoBase's AI understands all.</p>
              </div>
              <div className="neo-border bg-white p-4 sm:p-6">
                <h3 className="font-bold text-base sm:text-lg mb-2">✌️ Wide Range of Data Integrations</h3>
                <p className="text-sm sm:text-base text-gray-600">Setup various monitoring, analytical pipelines, databases, storage buckets and more</p>
              </div>
              <div className="neo-border bg-white p-4 sm:p-6">
                <h3 className="font-bold text-base sm:text-lg mb-2">👌 Full Infrastructure Control + Security</h3>
                <p className="text-sm sm:text-base text-gray-600">Deploy NeoBase on your own infrastructure - interact safely with AI Agents & LLMs</p>
              </div>
            </div>
          </div>
        </section>

        {/* Footer */}
        <Footer />
      </div>
    </>
  )
}

export default EnterprisePage