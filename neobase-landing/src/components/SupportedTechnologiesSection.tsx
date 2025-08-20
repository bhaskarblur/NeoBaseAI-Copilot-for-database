import React from 'react';
import FloatingBackground from './FloatingBackground';
import { Check, Clock, Database as DatabaseIcon, BrainCircuit } from 'lucide-react';

interface Technology {
  name: string;
  isSupported: boolean;
  priority?: number;
}

const SupportedTechnologiesSection: React.FC = () => {
  const databases: Technology[] = [
    { name: 'PostgreSQL', isSupported: true },
    { name: 'YugabyteDB', isSupported: true },
    { name: 'MongoDB', isSupported: true, priority: 1 },
    { name: 'MySQL', isSupported: true, priority: 2 },
    { name: 'Clickhouse', isSupported: true, priority: 5 },
    { name: 'Spreadsheet - CSV/XL', isSupported: true, priority: 7 },
    { name: 'Google Sheets', isSupported: true, priority: 7 },
    { name: 'Microsoft Excel', isSupported: false, priority: 8 }
  ];

  const llmClients: Technology[] = [
    { name: 'OpenAI', isSupported: true },
    { name: 'Google Gemini', isSupported: true },
    { name: 'Claude (Anthropic)', isSupported: false, priority: 1 },
    { name: 'Ollama (Local)', isSupported: false, priority: 2 }
  ];

  const TechnologyChip = ({ tech }: { tech: Technology }) => (
    <div className="neo-border bg-white px-3 sm:px-4 py-2 sm:py-3 flex items-center gap-2 sm:gap-3 transition-all hover:shadow-lg">
      <div className={`${tech.isSupported ? 'bg-green-100 text-green-600' : 'bg-amber-100 text-amber-500'} p-1 sm:p-1.5 rounded-full`}>
        {tech.isSupported ? <Check size={16} className="sm:w-4 sm:h-4" /> : <Clock size={16} className="sm:w-4 sm:h-4" />}
      </div>
      <span className="font-bold text-base sm:text-base">{tech.name}</span>
    </div>
  );

  return (
    <section id="technologies" className="py-12 sm:py-16 md:py-20 lg:py-24 bg-white relative overflow-hidden">
      <FloatingBackground count={18} opacity={0.03} />
      
      {/* Additional floating logos with different sizes */}
      <div className="absolute inset-0 -z-10 overflow-hidden">
        {Array.from({ length: 5 }).map((_, i) => (
          <img 
            key={`db-${i}`}
            src="/postgresql-logo.png" 
            alt="" 
            className="absolute w-8 sm:w-10 md:w-12 h-8 sm:h-10 md:h-12 animate-float opacity-10"
            style={{
              top: `${Math.random() * 100}%`,
              left: `${Math.random() * 30}%`,
              animationDelay: `${Math.random() * 10}s`,
              animationDuration: `${Math.random() * 10 + 20}s`,
              transform: `rotate(${Math.random() * 360}deg)`,
            }}
          />
        ))}
        
        {Array.from({ length: 3 }).map((_, i) => (
          <img 
            key={`mongo-${i}`}
            src="/mongodb-logo.svg" 
            alt="" 
            className="absolute w-10 sm:w-12 md:w-14 h-10 sm:h-12 md:h-14 animate-float opacity-10"
            style={{
              top: `${Math.random() * 100}%`,
              left: `${30 + Math.random() * 40}%`,
              animationDelay: `${Math.random() * 10}s`,
              animationDuration: `${Math.random() * 10 + 20}s`,
              transform: `rotate(${Math.random() * 360}deg)`,
            }}
          />
        ))}
        
        {Array.from({ length: 4 }).map((_, i) => (
          <img 
            key={`mysql-${i}`}
            src="/mysql-logo.png" 
            alt="" 
            className="absolute w-8 sm:w-9 md:w-10 h-8 sm:h-9 md:h-10 animate-float opacity-10"
            style={{
              top: `${Math.random() * 100}%`,
              left: `${70 + Math.random() * 30}%`,
              animationDelay: `${Math.random() * 10}s`,
              animationDuration: `${Math.random() * 10 + 20}s`,
              transform: `rotate(${Math.random() * 360}deg)`,
            }}
          />
        ))}
      </div>
      
      <div className="container mx-auto px-4 sm:px-6 md:px-8 max-w-7xl">
        <div className="text-center mb-8 sm:mb-12 md:mb-16">
          <h2 className="text-3xl sm:text-3xl md:text-4xl font-bold mb-3 md:mb-4">
            Supported <span className="text-green-500">Technologies</span>
          </h2>
          <p className="text-lg sm:text-lg text-gray-700 max-w-3xl mx-auto px-2">
            NeoBase works with a variety of Data sources and LLM clients, with more being added regularly.
          </p>
        </div>
        
        <div className="flex flex-col lg:flex-row gap-6 sm:gap-8 md:gap-10 lg:gap-16">
          {/* Databases Section */}
          <div className="flex-1">
            <div className="neo-border bg-[#FFDB58]/5 p-4 sm:p-5 md:p-6 h-full">
              <h3 className="text-xl sm:text-2xl font-bold mb-4 sm:mb-6 flex items-center">
                <DatabaseIcon className="mr-2 w-5 h-5 sm:w-6 sm:h-6" /> Data sources
              </h3>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
                {databases.map((db, index) => (
                  <TechnologyChip key={index} tech={db} />
                ))}
              </div>
            </div>
          </div>

          {/* LLM Clients Section */}
          <div className="flex-1">
            <div className="neo-border bg-[#FFDB58]/5 p-4 sm:p-5 md:p-6 h-full">
              <h3 className="text-xl sm:text-2xl font-bold mb-4 sm:mb-6 flex items-center">
                <BrainCircuit className="mr-2 w-5 h-5 sm:w-6 sm:h-6" /> LLM Clients
              </h3>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
                {llmClients.map((llm, index) => (
                  <TechnologyChip key={index} tech={llm} />
                ))}
              </div>
            </div>
          </div>
        </div>

        <div className="mt-6 sm:mt-8 text-center">
          <p className="text-base sm:text-base text-gray-600 italic">
            Don't see your database or LLM client? <a href="https://github.com/bhaskarblur/neobase-ai-dba/issues" className="text-green-600 hover:text-green-700 underline font-medium">Raise a request</a>
          </p>
        </div>
      </div>
    </section>
  );
};

export default SupportedTechnologiesSection; 