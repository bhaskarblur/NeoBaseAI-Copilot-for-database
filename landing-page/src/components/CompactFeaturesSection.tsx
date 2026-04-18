import React from 'react';
import { MessageSquare, Database, Zap, Shield, Server, Boxes, Rocket, Github } from 'lucide-react';
import FloatingBackground from './FloatingBackground';

const CompactFeaturesSection: React.FC<{ stars: number }> = ({ stars }) => {


  const features = [
    {
      icon: <MessageSquare className="w-6 h-6 md:w-8 md:h-8" />,
      title: "AI-Powered Conversations & Insights",
      description: "Ask questions, get answers, and manage your datasources with natural language.",
      width: "md:col-span-2 lg:col-span-2",
      transform: "rotate(-1deg)",
      importance: "primary",
      bgColor: "bg-[#FFDB58]/20"
    },
    {
      icon: <Database className="w-5 h-5 md:w-6 md:h-6" />,
      title: "Multi-Data Source Support",
      description: "Connect to PostgreSQL, YugabyteDB, MySQL, MongoDB, Redis, Neo4j and more.",
      width: "md:col-span-1 lg:col-span-1",
      transform: "rotate(1deg)",
      importance: "secondary",
      bgColor: "bg-blue-100/50"
    },
    {
      icon: <Zap className="w-5 h-5 md:w-7 md:h-7" />,
      title: "Quick, Accurate Insights in Real-time",
      description: "Get AI-driven suggestions to improve database performance.",
      width: "md:col-span-1 lg:col-span-1",
      transform: "rotate(-0.5deg)",
      importance: "secondary",
      bgColor: "bg-green-100/50"
    },
    {
      icon: <Shield className="w-6 h-6 md:w-8 md:h-8" />,
      title: "Self-Hosted & Open Source",
      description: "NeoBase gives you the ultimate control. Choose the LLM client of your choice & deploy on your own infrastructure. No data leaves your infrastructure unless you want it to.",
      width: "md:col-span-2 lg:col-span-2",
      transform: "rotate(0.7deg)",
      importance: "secondary",
      bgColor: "bg-purple-100/50"
    },
    {
      icon: <Server className="w-5 h-5 md:w-6 md:h-6" />,
      title: "Query Execution & Data Management",
      description: "Execute queries, rollback if needed, and visualize large volumes of data with ease.",
      width: "md:col-span-2 lg:col-span-2",
      transform: "rotate(-0.8deg)",
      importance: "primary",
      bgColor: "bg-orange-100/50"
    },
    {
      icon: <Boxes className="w-5 h-5 md:w-6 md:h-6" />,
      title: "Smart Decision Making",
      description: "NeoBase manages your data smartly for you & enables you to take decisions and act smarter. ",
      width: "md:col-span-1 lg:col-span-1",
      transform: "rotate(0.5deg)",
      importance: "secondary",
      bgColor: "bg-red-100/50"
    }
  ];


  const formatStarCount = (count: number): string => {
    if (count >= 1000) {
      return `${(count / 1000).toFixed(1)}k`
    }
    return count.toString()
  }

  return (
    <section id="features" className="py-12 sm:py-16 md:py-20 lg:py-24 bg-[#FFDB58]/5 relative overflow-hidden">
      <FloatingBackground count={10} opacity={0.03} />

      <div className="container mx-auto px-4 sm:px-6 md:px-8 max-w-7xl">
        <div className="text-center mb-8 md:mb-12">
          <h2 className="text-3xl sm:text-3xl md:text-4xl font-bold mb-3 md:mb-4">
            <span className="text-yellow-500">Features</span> Your Data deserves
          </h2>
          <p className="text-lg sm:text-lg text-gray-700 max-w-3xl mx-auto px-2">
            NeoBase makes data analysis simple and intuitive with these powerful features.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-3 gap-4 sm:gap-5 md:gap-6 lg:gap-8">
          {features.map((feature, index) => (
            <div
              key={index}
              className={`neo-border bg-white p-4 sm:p-4 md:p-5 hover:shadow-lg transition-all duration-300 ${feature.width}`}
              style={{
                transform: feature.transform,
                zIndex: feature.importance === "primary" ? 10 : feature.importance === "secondary" ? 5 : 1
              }}
            >
              <div className={`flex ${feature.importance === "primary" ? "flex-col items-start" : "items-start"}`}>
                <div className={`${feature.bgColor} p-3 sm:p-3 rounded-lg ${feature.importance === "primary" ? "mb-3 md:mb-4" : "mr-3 md:mr-4"} ${index % 2 === 0 ? 'self-start' : 'self-center'}`}>
                  {feature.icon}
                </div>
                <div>
                  <h3 className={`font-bold mb-2 sm:mb-2 ${feature.importance === "primary" ? "text-xl sm:text-xl" : feature.importance === "secondary" ? "text-lg sm:text-lg" : "text-base sm:text-base"}`}>
                    {feature.title}
                  </h3>
                  <p className={`text-gray-600 ${feature.importance === "primary" ? "text-base sm:text-base" : "text-sm sm:text-sm"}`}>
                    {feature.description}
                  </p>
                </div>
              </div>
            </div>
          ))}
        </div>

        <div className="flex flex-col md:flex-row w-full justify-center gap-4 mt-8 sm:mt-12 md:mt-16 text-center ">
          <a
            href={import.meta.env.VITE_NEOBASE_APP_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="neo-button inline-flex items-center justify-center gap-2 py-3 px-6 sm:py-3 sm:px-8 text-lg sm:text-lg"
          >
            <span className="flex items-center gap-2 sm:gap-4"><Rocket className="w-5 h-5 sm:w-5 sm:h-5" /> Try NeoBase</span>
          </a>
          <a
            href="https://github.com/bhaskarblur/neobase-ai-dba"
            target="_blank"
            rel="noopener noreferrer"
            className="neo-button-secondary flex items-center justify-center gap-2 py-3 sm:py-3 px-5 sm:px-6 text-base sm:text-lg"
          >
            <Github className="w-4 h-4 sm:w-5 sm:h-5" />
            <span>Star Us</span>
            <span className="bg-black/20 px-2 py-0.5 rounded-full text-xs font-mono">
              {formatStarCount(stars || 1)}
            </span>
          </a>
        </div>
      </div>
    </section>
  );
};

export default CompactFeaturesSection; 