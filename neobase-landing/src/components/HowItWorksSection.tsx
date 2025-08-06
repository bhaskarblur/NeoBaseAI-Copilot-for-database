import React from 'react';
import { MessageSquare, Database, Zap, Rocket, Server } from 'lucide-react';
import FloatingBackground from './FloatingBackground';

const HowItWorksSection: React.FC = () => {
  const steps = [
    {
        icon: <Database className="w-6 h-6" />,
        title: "Connect to Your Data Source",
        description: "NeoBase connects securely to your database, whether it's PostgreSQL, MySQL, MongoDB, or others, keeping your data safe."
      },
    {
      icon: <MessageSquare className="w-6 h-6" />,
      title: "Ask in The Language You Speak",
      description: "Simply ask NeoBase what you want to do with your database using everyday language. No need to remember complex SQL syntax."
    },
    {
      icon: <Zap className="w-6 h-6" />,
      title: "AI Generates Optimized Queries",
      description: "The AI analyzes your request and generates the most efficient database queries, optimized for performance."
    },
    {
      icon: <Rocket className="w-6 h-6" />,
      title: "Execute and Visualize Results",
      description: "Review the generated queries, execute them with a click, and see your results in a clean, visual format."
    }
  ];

  return (
    <section id="how-it-works" className="py-12 md:py-20 lg:py-24 bg-white relative overflow-hidden">
      <FloatingBackground count={6} opacity={0.02} />
      
      <div className="container mx-auto px-4 sm:px-6 md:px-8 max-w-7xl">
        <div className="text-center mb-8 md:mb-16">
          <h2 className="text-3xl sm:text-3xl md:text-4xl font-bold mb-3 md:mb-4">
            How <span className="text-green-500">NeoBase</span> Works
          </h2>
          <p className="text-lg md:text-lg text-gray-700 max-w-3xl mx-auto px-2">
            From human language to database results in seconds
          </p>
        </div>
        
        <div className="flex flex-col lg:flex-row items-center gap-8 md:gap-12">
          {/* Steps on the left */}
          <div className="w-full lg:w-1/2">
            <div className="space-y-4 sm:space-y-6 md:space-y-8">
              {steps.map((step, index) => (
                <div 
                  key={index} 
                  className="flex gap-3 md:gap-4 neo-border p-3 sm:p-4 md:p-5 bg-white hover:shadow-md transition-all duration-300"
                  style={{ transform: `rotate(${index % 2 === 0 ? '-0.4deg' : '0.4deg'})` }}
                >
                  <div className="flex-shrink-0 bg-[#FFDB58]/20 p-2 md:p-3 rounded-lg self-start">
                    {step.icon}
                  </div>
                  <div>
                    <div className="flex items-center gap-2 mb-1 md:mb-2">
                      <div className="w-5 h-5 md:w-6 md:h-6 rounded-full bg-black text-white flex items-center justify-center text-xs md:text-sm font-bold">
                        {index + 1}
                      </div>
                      <h3 className="font-bold text-lg md:text-xl">{step.title}</h3>
                    </div>
                    <p className="text-base md:text-base text-gray-600">{step.description}</p>
                  </div>
                </div>
              ))}
              <div className="mt-8 md:mt-12 lg:mt-16 text-center">
                <a 
                  href="https://github.com/bhaskarblur/neobase-ai-dba" 
                  target="_blank" 
                  rel="noopener noreferrer" 
                  className="neo-button inline-flex items-center justify-center gap-2 py-2 px-6 md:py-3 md:px-8 text-base md:text-lg"
                >
                  <span className="flex items-center gap-2 md:gap-4"><Server className="w-4 h-4 md:w-5 md:h-5" /> Start Setup Now</span>
                </a>
              </div>
            </div>
          </div>
          
          {/* Image on the right */}
          <div className="w-full lg:w-1/2 flex justify-center mt-8 lg:mt-0">
            <div className="neo-border p-2 bg-white max-w-[90%] sm:max-w-[80%] md:max-w-[500px]" style={{ transform: 'rotate(0.5deg)' }}>
              <div className="neo-border p-2 sm:p-3 md:p-4 bg-[#FFDB58]/5 overflow-hidden">
                <img 
                  src="/working-1.png" 
                  alt="NeoBase in action" 
                  className="w-full h-auto rounded-lg shadow-lg"
                />
                <div className="mt-3 md:mt-4 p-3 md:p-4 bg-white rounded-lg neo-border">
                  <div className="flex items-center gap-2 mb-3 md:mb-4">
                    <MessageSquare className="w-4 h-4 md:w-5 md:h-5 text-green-500" />
                    <p className="text-base md:text-base text-gray-700 italic">"Talk to your data"</p>
                  </div>
                  <img src="/working-2.png" alt="NeoBase in action" className="w-full h-auto rounded-lg shadow-lg" />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default HowItWorksSection; 