import { Github } from 'lucide-react';
import FloatingBackground from './FloatingBackground';

const Footer = () => {
  const currentYear = new Date().getFullYear();

  return (
    <footer className="bg-black text-white py-12 relative overflow-hidden">
      <FloatingBackground count={8} opacity={0.1} />
      <div className="container mx-auto px-6 md:px-8 lg:px-0 max-w-7xl">
        <div className="flex flex-col md:flex-row justify-between items-center">
          <div className="flex items-center mb-6 md:mb-0">
            <img src="/neobase-logo.svg" alt="NeoBase Logo" className="w-10 h-10 filter invert" />
            <span className="text-2xl font-bold ml-2">NeoBase</span>
          </div>
          
          <div className="flex flex-col md:flex-row items-center gap-4 md:gap-8">
            <a 
              href="https://github.com/bhaskarblur/neobase-ai-dba" 
              target="_blank" 
              rel="noopener noreferrer"
              className="flex items-center gap-2 hover:text-[#FFDB58] transition-colors"
            >
              <Github className="w-5 h-5" />
              <span>GitHub</span>
            </a>
            <a 
              href="https://discord.gg/VT9NRub86D" // Official Discord Server
              target="_blank" 
              rel="noopener noreferrer"
              className="flex items-center gap-2 hover:text-[#5865F2] transition-colors group"
            >
              <img src="https://uxwing.com/wp-content/themes/uxwing/download/brands-and-social-media/discord-white-icon.png" alt="Discord" className="w-5 h-5 transition-all group-hover:[filter:brightness(0)_saturate(100%)_invert(37%)_sepia(68%)_saturate(1920%)_hue-rotate(220deg)_brightness(99%)_contrast(95%)]" />
              <span>Discord</span>
            </a>
            <a 
              href={import.meta.env.VITE_PRODUCT_HUNT_URL}
              target="_blank" 
              rel="noopener noreferrer"
              className="flex items-center gap-2 hover:text-[#DA552F] transition-colors"
            >
              <svg className="w-5 h-5" viewBox="0 0 512 512" fill="currentColor">
                <path d="M256 0C114.615 0 0 114.615 0 256s114.615 256 256 256 256-114.615 256-256S397.385 0 256 0zm-96 320h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32zm64 160h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32zm32 160V128h32v96h32v96h-64zm96 0h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32V80h32v32zm64 240h-32v-32h32v32zm0-80h-32v-32h32v32zm0-80h-32v-32h32v32z"/>
              </svg>
              <span>Product Hunt</span>
            </a>
            <a 
              href="https://github.com/bhaskarblur/neobase-ai-dba/issues" 
              target="_blank" 
              rel="noopener noreferrer"
              className="hover:text-[#FFDB58] transition-colors"
            >
              Report an Issue
            </a>
            <a 
              href="https://github.com/bhaskarblur/neobase-ai-dba/blob/main/LICENSE.md" 
              target="_blank" 
              rel="noopener noreferrer"
              className="hover:text-[#FFDB58] transition-colors"
            >
              License
            </a>
          </div>
        </div>
        
        <div className="border-t border-gray-800 mt-8 pt-8">
          <div className="flex flex-col md:flex-row justify-between items-center">
            <p className="text-gray-400">
              &copy; {currentYear} NeoBase - AI Copilot for Database
            </p>
            <div className="flex items-center text-gray-400 mt-4 md:mt-0">
              <span className="mr-1">Crafted with ❤️  by </span>
              <a 
                href="https://www.linkedin.com/in/ankit-apk/" 
                target="_blank" 
                rel="noopener noreferrer"
                className="hover:text-yellow-400 transition-colors flex items-center"
              >
                <span>Ankit</span>
              </a>
              <span className="mx-1">&</span>
              <a 
                href="https://www.linkedin.com/in/bhaskarkaura07/" 
                target="_blank" 
                rel="noopener noreferrer"
                className="hover:text-green-400 transition-colors flex items-center"
              >
                <span> Bhaskar</span>
              </a>
            </div>
          </div>
        </div>
      </div>
    </footer>
  );
};

export default Footer; 