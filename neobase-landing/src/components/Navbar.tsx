import { Boxes, Github, Menu, X } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'

const Navbar = ({ forks }: { forks: number }) => {
  const [isMenuOpen, setIsMenuOpen] = useState(false)


  const formatForkCount = (count: number): string => {
    if (count >= 1000) {
      return `${(count / 1000).toFixed(1)}k`
    }
    return count.toString()
  }


  const toggleMenu = () => {
    setIsMenuOpen(!isMenuOpen)
  }

  const handleSmoothScroll = (e: React.MouseEvent<HTMLAnchorElement>, targetId: string) => {
    e.preventDefault()
    const element = document.querySelector(targetId)
    if (element) {
      const offset = 80 // Account for fixed navbar height
      const elementPosition = element.getBoundingClientRect().top
      const offsetPosition = elementPosition + window.pageYOffset - offset

      window.scrollTo({
        top: offsetPosition,
        behavior: 'smooth'
      })
    }
    // Close mobile menu if open
    if (isMenuOpen) {
      setIsMenuOpen(false)
    }
  }

  return (
    <>
      <nav className="py-4 px-6 md:px-8 lg:px-12 border-b-4 border-black bg-white fixed top-0 left-0 right-0 z-[100] shadow-md">
        <div className="container mx-auto max-w-7xl">
          <div className="flex items-center justify-between">
            {/* Logo */}
            <Link to="/" className="flex items-center gap-2">
              <Boxes className="w-8 h-8" />
              <span className="text-2xl font-bold">NeoBase</span>
            </Link>

            {/* Desktop Navigation */}
            <div className="hidden md:flex items-center gap-6">
              <Link to="/enterprise" className="font-medium text-yellow-600 hover:text-yellow-800 transition-colors">Enterprise</Link>
              <a href="#features" onClick={(e) => handleSmoothScroll(e, '#features')} className="font-medium hover:text-gray-600 transition-colors cursor-pointer">Features</a>
              <a href="#technologies" onClick={(e) => handleSmoothScroll(e, '#technologies')} className="font-medium hover:text-gray-600 transition-colors cursor-pointer">Technologies</a>
              <a href="#use-cases" onClick={(e) => handleSmoothScroll(e, '#use-cases')} className="font-medium hover:text-gray-600 transition-colors cursor-pointer">Use Cases</a>
              
              {/* Product Hunt Button */}
              <a href="https://www.producthunt.com/posts/neobase-2?embed=true&utm_source=badge-featured&utm_medium=badge&utm_souce=badge-neobase&#0045;2" target="_blank"><img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=936307&theme=light&t=1741073867985" alt="NeoBase - AI&#0032;powered&#0032;database&#0032;assistant | Product Hunt" style={{width: '220px', height: '48px'}} width="220" height="48" /></a>
              
              {/* Github Fork Button */}
              <a 
                href="https://github.com/bhaskarblur/neobase-ai-dba" 
                target="_blank" 
                rel="noopener noreferrer"
                className="neo-button flex items-center gap-2 py-2 px-4 text-sm bg-black text-white"
              >
                <Github className="w-4 h-4" />
                <span>Fork Us</span>
                <span className="bg-white/20 px-2 py-0.5 rounded-full text-xs font-mono">
                  {formatForkCount(forks || 1)}
                </span>
              </a>
              
            </div>

            {/* Mobile Menu Button */}
            <button 
              className="md:hidden p-2 neo-border bg-white"
              onClick={toggleMenu}
              aria-label="Toggle menu"
            >
              {isMenuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
            </button>
          </div>

          {/* Mobile Navigation */}
          {isMenuOpen && (
            <div className="md:hidden mt-4 py-4 border-t border-gray-200">
              <div className="flex flex-col gap-4">
                <Link to="/enterprise" onClick={() => setIsMenuOpen(false)} className="font-medium text-yellow-600 hover:text-yellow-800 transition-colors py-2">Enterprise</Link>
                <a href="#features" onClick={(e) => handleSmoothScroll(e, '#features')} className="font-medium hover:text-gray-600 transition-colors py-2 cursor-pointer">Features</a>
                <a href="#technologies" onClick={(e) => handleSmoothScroll(e, '#technologies')} className="font-medium hover:text-gray-600 transition-colors py-2 cursor-pointer">Technologies</a>
                <a href='#use-cases' onClick={(e) => handleSmoothScroll(e, '#use-cases')} className="font-medium hover:text-gray-600 transition-colors py-2 cursor-pointer">Use-Cases</a>
                
                <div className="flex flex-col gap-3 mt-2">
                  {/* Product Hunt Button */}
                  <a href="https://www.producthunt.com/posts/neobase-2?embed=true&utm_source=badge-featured&utm_medium=badge&utm_souce=badge-neobase&#0045;2" target="_blank"><img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=936307&theme=light&t=1741073867985" alt="NeoBase - AI&#0032;powered&#0032;database&#0032;assistant | Product Hunt" style={{ height: '48px'}}  height="48" /></a>
                  
                  {/* Github Fork Button */}
                  <a 
                    href="https://github.com/bhaskarblur/neobase-ai-dba" 
                    target="_blank" 
                    rel="noopener noreferrer"
                    className="neo-button flex items-center justify-center gap-2 py-2 bg-black text-white"
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
          )}
        </div>
      </nav>
      {/* Spacer to prevent content from being hidden under the fixed navbar */}
      <div className="h-[73px]"></div>
    </>
  )
}

export default Navbar 