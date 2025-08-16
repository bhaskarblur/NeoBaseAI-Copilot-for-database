import { ArrowRight, Boxes, Github } from 'lucide-react'
import { useState } from 'react'
import FloatingBackground from './FloatingBackground'

const HeroSection = () => {
  const [viewMode, setViewMode] = useState<'technical' | 'non-technical'>('technical')
  
  return (
    <section className="py-12 sm:py-16 md:py-20 lg:py-28 relative">
      {/* Background Pattern */}
      <FloatingBackground count={20} opacity={0.05} />

      <div className="container mt-4 sm:mt-6 md:mt-8 mx-auto px-4 sm:px-6 md:px-0 relative max-w-7xl">
        <div className="flex flex-col md:flex-row items-center">
          {/* Left Side - Hero Text */}
          <div className="md:w-1/2 space-y-4 sm:space-y-5 md:space-y-6 md:pr-8 z-10">
            <div className="inline-block neo-border bg-[#FFDB58] px-3 sm:px-4 py-1.5 sm:py-2 font-bold text-sm sm:text-sm">
              #Open-Source,  #AI Copilot for Database
            </div>
            <h1 className="text-5xl sm:text-5xl md:text-5xl lg:text-6xl font-extrabold leading-tight">
              Where there's a Database.<br />
              <span className="text-yellow-500">There's <span className="text-green-500">NeoBase!</span></span>
            </h1>
            <p className="text-lg sm:text-lg md:text-xl text-gray-700">
              NeoBase connects to your database & let's you talk to your data. No boring dashboards anymore, just you & your data.
            </p>
            <div className="flex flex-col sm:flex-row gap-3 sm:gap-4 pt-4 sm:pt-6">
              <a 
                href={import.meta.env.VITE_NEOBASE_APP_URL}
                target="_blank" 
                rel="noopener noreferrer" 
                className="neo-button flex items-center justify-center gap-2 py-2 sm:py-3 px-6 sm:px-8 text-base sm:text-lg"
              >
                <Boxes className="w-4 h-4 sm:w-5 sm:h-5" />
                <span>Try NeoBase - It's Free!</span>
              </a>
              <a 
                href="https://github.com/bhaskarblur/neobase-ai-dba" 
                target="_blank" 
                rel="noopener noreferrer" 
                className="neo-button-secondary flex items-center justify-center gap-2 py-2 sm:py-3 px-5 sm:px-6 text-base sm:text-lg"
              >
                <Github className="w-4 h-4 sm:w-5 sm:h-5" />
                <span>View on GitHub</span>
              </a>
            </div>
          </div>

          {/* Right Side - Hero Image */}
          <div className="md:w-7/12 mt-10 sm:mt-12 md:mt-40 md:absolute md:right-0 md:translate-x-[10%] lg:translate-x-[15%] xl:translate-x-[20%] z-20" >
            <div className="neo-border bg-white p-0 mx-0 sm:mx-6 md:mx-0 relative hover:shadow-lg transition-all duration-300">
              <div className="relative overflow-hidden">
                <img 
                  src="/hero-ss.png" 
                  alt="NeoBase Chat Technical View" 
                  className={`w-full h-auto transition-opacity duration-500 ${viewMode === 'technical' ? 'opacity-100' : 'opacity-0 absolute top-0 left-0'}`} 
                />
                <img 
                  src="/hero-ss-nontech.png" 
                  alt="NeoBase Chat Non-Technical View" 
                  className={`w-full h-auto transition-opacity duration-500 ${viewMode === 'non-technical' ? 'opacity-100' : 'opacity-0 absolute top-0 left-0'}`} 
                />
              </div>
              {/* Toggle Buttons */}
              <div className="flex gap-2 mt-4 justify-center pb-4 relative z-30">
                <button
                  onClick={() => setViewMode('technical')}
                  className={`px-4 py-2 font-semibold transition-all duration-200 cursor-pointer ${
                    viewMode === 'technical' 
                      ? 'bg-black text-white neo-border' 
                      : 'bg-white text-black neo-border hover:bg-gray-100'
                  }`}
                >
                  <span className="text-sm md:text-base">View Technical</span>
                </button>
                <button
                  onClick={() => setViewMode('non-technical')}
                  className={`px-4 py-2 font-semibold transition-all duration-200 cursor-pointer ${
                    viewMode === 'non-technical' 
                      ? 'bg-black text-white neo-border' 
                      : 'bg-white text-black neo-border hover:bg-gray-100'
                  }`}
                >
                 <span className="text-sm md:text-base">View Non-Technical</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default HeroSection; 