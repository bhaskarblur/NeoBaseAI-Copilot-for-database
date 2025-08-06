import { Github } from 'lucide-react'
import FloatingBackground from './FloatingBackground'

const ContributeSection = () => {
  return (
    <section id="contribute" className="py-12 sm:py-16 md:py-20 lg:py-28 relative">
      <FloatingBackground count={15} opacity={0.05} />
      
      <div className="container mx-auto px-4 sm:px-6 md:px-0 relative max-w-7xl">
        <div className="text-center max-w-4xl mx-auto">
          <div className="inline-block neo-border bg-[#FFDB58] px-3 sm:px-4 py-1.5 sm:py-2 font-bold text-sm sm:text-sm mb-6">
            #Community #Open-Source
          </div>
          
          <h2 className="text-3xl sm:text-3xl md:text-4xl lg:text-5xl font-extrabold leading-tight mb-6">
            Want To Contribute?
          </h2>
          
          <p className="text-lg sm:text-lg md:text-xl text-gray-700 mb-8 max-w-3xl mx-auto">
            Join our growing community and help us make NeoBase Open-source even better! 
            We'd love to have you aboard and hear from you about features, bug report, enhancements & more.
          </p>
          
          <div className="flex flex-col sm:flex-row gap-3 sm:gap-4 justify-center items-center">
            <a
              href="https://discord.gg/VT9NRub86D" // Official Discord Server
              target="_blank"
              rel="noopener noreferrer"
              className="neo-button flex items-center justify-center gap-2 py-2 sm:py-3 px-6 sm:px-8 text-base sm:text-lg bg-[#5865F2] border-[#5865F2]"
            >
              {/* <MessageCircle className="w-4 h-4 sm:w-5 sm:h-5" /> */}
              <img src="https://uxwing.com/wp-content/themes/uxwing/download/brands-and-social-media/discord-white-icon.png" alt="Discord" className="w-5 h-5 sm:w-5 sm:h-5" />
              <span>Join us on Discord</span>
            </a>
            
            <a
              href="https://github.com/bhaskarblur/neobase-ai-dba"
              target="_blank"
              rel="noopener noreferrer"
              className="neo-button-secondary flex items-center justify-center gap-2 py-2 sm:py-3 px-5 sm:px-6 text-base sm:text-lg"
            >
              <Github className="w-4 h-4 sm:w-5 sm:h-5" />
              <span>View our GitHub</span>
            </a>
          </div>
          
          <div className="mt-8 text-center">
            <p className="text-gray-600 text-base">
              ⭐ Star our repository to stay updated with the latest features and improvements!
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}

export default ContributeSection