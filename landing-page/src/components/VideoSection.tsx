import { Maximize, Pause, Play, Volume2, VolumeX } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';
import FloatingBackground from './FloatingBackground';

const VideoSection = () => {
  const [isPlaying, setIsPlaying] = useState(false);
  const [isMuted, setIsMuted] = useState(true);
  const [isVideoLoaded, setIsVideoLoaded] = useState(false);
  const videoRef = useRef<HTMLVideoElement>(null);

  const togglePlayPause = () => {
    if (videoRef.current) {
      if (isPlaying) {
        videoRef.current.pause();
      } else {
        videoRef.current.play();
      }
      setIsPlaying(!isPlaying);
    }
  };

  const toggleMute = () => {
    if (videoRef.current) {
      videoRef.current.muted = !videoRef.current.muted;
      setIsMuted(!isMuted);
    }
  };

  const toggleFullScreen = () => {
    if (videoRef.current) {
      if (document.fullscreenElement) {
        document.exitFullscreen();
      } else {
        videoRef.current.requestFullscreen();
      }
    }
  };

  useEffect(() => {
    const video = videoRef.current;
    if (video) {
      const handleLoadedData = () => {
        setIsVideoLoaded(true);
      };
      
      video.addEventListener('loadeddata', handleLoadedData);
      
      return () => {
        video.removeEventListener('loadeddata', handleLoadedData);
      };
    }
  }, []);

  return (
    <section className="py-16 sm:py-16 md:py-24 lg:py-36  relative overflow-hidden bg-[#FFDB58]/5 mb-16 sm:mb-24 md:mb-32 lg:mb-36">
      <FloatingBackground count={15} opacity={0.03} />
      <div className="container mx-auto px-2 sm:px-6 md:px-8 md:mt-8 relative">
        <h2 className="pt-0 md:pt-24 text-3xl sm:text-3xl md:text-4xl lg:text-4xl font-bold text-center mb-6 sm:mb-10 md:mb-16">
          Connect to your Data Source & <span className="text-yellow-500">Get Started</span>
        </h2>
        
        <div className="relative">
          {/* Video Container */}
          <div className="max-w-5xl mx-auto relative z-10">
            <div className="neo-border bg-white p-1 sm:p-2 md:p-4 hover:shadow-xl transition-all duration-300 group" style={{ transform: 'rotate(-0.5deg)' }}>
              {/* Video */}
              <div className="relative overflow-hidden min-h-[350px] sm:min-h-[350px] md:min-h-[450px] lg:min-h-[560px] rounded-lg bg-black">
                {/* Poster Image (shown until video loads) */}
                {!isVideoLoaded && (
                  <div className="absolute inset-0 z-0">
                    <img 
                      src="/video-poster.png" 
                      alt="Video thumbnail" 
                      className="w-full h-full object-cover rounded-lg"
                    />
                  </div>
                )}
                
                <video 
                  ref={videoRef}
                  className="w-full h-full object-cover absolute inset-0 rounded-lg"
                  poster=""
                  muted={isMuted}
                >
                  <source src={import.meta.env.VITE_DEMO_VIDEO_URL} type="video/mp4" />
                  Your browser does not support the video tag.
                </video>
                
                {/* Play/Pause Button */}
                <button 
                  onClick={togglePlayPause}
                  className={`absolute top-1/2 left-1/2 transform -translate-x-1/2 -translate-y-1/2 bg-[#FFDB58] hover:bg-[#FFDB58]/90 text-black neo-border rounded-full p-4 sm:p-5 md:p-6 transition-all ${isPlaying ? 'opacity-0 group-hover:opacity-100 focus:opacity-100' : 'opacity-100'}`}
                  aria-label={isPlaying ? "Pause video" : "Play video"}
                >
                  {isPlaying ? (
                    <Pause className="w-7 h-7 sm:w-8 sm:h-8 md:w-10 md:h-10" />
                  ) : (
                    <Play className="w-7 h-7 sm:w-8 sm:h-8 md:w-10 md:h-10 ml-0.5 sm:ml-1" />
                  )}
                </button>
                
                {/* Mute/Unmute Button */}
                <button 
                  onClick={toggleMute}
                  className="absolute top-3 right-14 sm:top-4 sm:right-16 md:top-5 md:right-16 bg-black/70 hover:bg-black/80 text-white rounded-full p-1.5 sm:p-2 transition-all"
                  aria-label={isMuted ? "Unmute video" : "Mute video"}
                >
                  {isMuted ? (
                    <VolumeX className="w-4 h-4 sm:w-5 sm:h-5" />
                  ) : (
                    <Volume2 className="w-4 h-4 sm:w-5 sm:h-5" />
                  )}
                </button>

                {/* Full Screen Button */}
                <button 
                  onClick={toggleFullScreen}
                  className="absolute top-3 right-3 sm:top-4 sm:right-4 md:top-5 md:right-5 bg-black/70 hover:bg-black/80 text-white rounded-full p-1.5 sm:p-2 transition-all"
                  aria-label="Toggle full screen"
                >
                  <Maximize className="w-4 h-4 sm:w-5 sm:h-5" />
                </button>
              </div>
            </div>
          </div>
          
          {/* Feature Callouts with Arrows */}
          
          {/* Right Feature */}
          <div className="absolute right-0 top-1/2 transform translate-x-[20%] -translate-y-1/2 z-0 hidden md:block">
            <div className="flex items-center">
              <svg width="120" height="100" viewBox="0 0 120 100" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M120 50 L0 50" stroke="black" strokeWidth="6" fill="none" strokeLinecap="round"/>
                <polygon points="0,50 25,35 25,65" fill="black"/>
              </svg>
              <div className="neo-border bg-[#FFDB58] px-4 sm:px-6 py-2 sm:py-3 font-bold text-lg md:text-xl text-center ml-4 sm:ml-6 whitespace-nowrap">
                Visualize & Export Results
              </div>
            </div>
          </div>
          
          {/* Bottom Feature */}
          <div className=" absolute bottom-0 left-1/2 transform -translate-x-1/2 translate-y-[60%] z-0 hidden md:block md:mb-4">
            <div className="flex flex-col items-center">
              <svg width="100" height="120" viewBox="0 0 100 120" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M50 120 L50 0" stroke="black" strokeWidth="6" fill="none" strokeLinecap="round"/>
                <polygon points="50,0 35,25 65,25" fill="black"/>
              </svg>
              <div className="neo-border bg-[#FFDB58] px-4 sm:px-6 py-2 sm:py-3 font-bold text-lg md:text-xl text-center mt-4 sm:mt-6 whitespace-nowrap">
                Talk to Your Database Naturally
              </div>
            </div>
          </div>
          
          {/* Left Feature */}
          <div className="absolute left-0 top-1/2 transform -translate-x-[20%] -translate-y-1/2 z-0 hidden md:block">
            <div className="flex items-center">
              <div className="neo-border bg-[#FFDB58] px-4 sm:px-6 py-2 sm:py-3 font-bold text-lg md:text-xl text-center mr-4 sm:mr-6 whitespace-nowrap">
                AI-Driven Operations
              </div>
              <svg width="120" height="100" viewBox="0 0 120 100" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M0 50 L120 50" stroke="black" strokeWidth="6" fill="none" strokeLinecap="round"/>
                <polygon points="120,50 95,35 95,65" fill="black"/>
              </svg>
            </div>
          </div>
          
          {/* Mobile Features (simplified) */}
          <div className="md:hidden space-y-4 mt-8 sm:mt-10">
            <div className="neo-border bg-[#FFDB58] px-4 py-3 sm:py-3 font-bold text-lg sm:text-lg text-center mx-2 sm:mx-4">
              AI-Driven Operations
            </div>
            <div className="neo-border bg-[#FFDB58] px-4 py-3 sm:py-3 font-bold text-lg sm:text-lg text-center mx-2 sm:mx-4">
              Visualize & Export Results
            </div>
            <div className="neo-border bg-[#FFDB58] px-4 py-3 sm:py-3 font-bold text-lg sm:text-lg text-center mx-2 sm:mx-4">
              Talk to Your Database Naturally
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default VideoSection; 