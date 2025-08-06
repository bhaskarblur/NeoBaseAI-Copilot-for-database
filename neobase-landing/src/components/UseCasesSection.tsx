import React, { useState } from 'react';
import { Code, PieChart, Users, Database, Briefcase, LayoutDashboard, LineChart } from 'lucide-react';
import FloatingBackground from './FloatingBackground';

interface UseCase {
  title: string;
  icon: React.ReactNode;
  description: string;
  examples: string[];
  benefits: string[];
  result: string;
  query: string;
}

const UseCasesSection: React.FC = () => {
  const [activeTab, setActiveTab] = useState(0);

  const useCases: UseCase[] = [
    {
        title: "Software Developer",
        icon: <Code className="w-6 h-6 text-green-600" />,
        description: "Developers can quickly inspect and update database records without writing complex SQL queries.",
        examples: [
          "Debug application issues by querying relevant data",
          "Check database state during development",
          "Validate database migrations and schema changes"
        ],
        benefits: [
          "Save time writing complex queries",
          "Reduce context switching between code and SQL",
          "Faster debugging and issue resolution"
        ],
        query: "Show me all users who registered in the last week but haven't confirmed their email",
        result: "A list of users who registered in the last week but haven't confirmed their email"
      },
    {
      title: "Data Analyst",
      icon: <PieChart className="w-6 h-6 text-blue-600" />,
      description: "Data analysts can explore datasets and generate insights without needing SQL expertise.",
      examples: [
        "Generate reports on user engagement metrics",
        "Analyze sales trends across different time periods",
        "Identify correlations between product usage and retention"
      ],
      benefits: [
        "Democratize data access across the organization",
        "Generate visualizations directly from natural language",
        "Faster time-to-insight for business decisions"
      ],
      query: "What's the average order value by country for the last quarter, sorted by highest first?",
      result: "A list of countries with the average order value for the last quarter, sorted by highest first"
    },
    {
        title: "C-Suite Executive",
        icon: <LineChart className="w-6 h-6 text-red-600" />,
        description: "Executives can access critical business insights instantly without relying on multiple teams and reports.",
        examples: [
          "Monitor key performance indicators across the entire business",
          "Analyze growth trends and identify emerging opportunities",
          "Track progress on strategic initiatives in real-time"
        ],
        benefits: [
          "Make data-driven decisions with up-to-the-minute information",
          "Break down data silos across departments",
          "Reduce time from question to insight from days to seconds"
        ],
        query: "Show me revenue growth by product line compared to last year, with customer acquisition costs",
        result: "A comprehensive breakdown of revenue growth by product line with year-over-year comparison and CAC metrics"
      },
    {
      title: "Product Manager",
      icon: <Users className="w-6 h-6 text-purple-600" />,
      description: "Product managers can directly access user data to inform product decisions without technical barriers.",
      examples: [
        "Track feature adoption rates",
        "Analyze user journeys and conversion funnels",
        "Monitor key product metrics over time"
      ],
      benefits: [
        "Self-service data access without engineering dependency",
        "Faster validation of product hypotheses",
        "Data-driven feature prioritization"
      ],
      query: "Show me the conversion rate for our new checkout flow compared to the old one over the past month",
      result: "Conversion rates for the new and old checkout flows over the past month"
    },
    {
      title: "Business Analyst",
      icon: <Briefcase className="w-6 h-6 text-yellow-600" />,
      description: "Business users can extract actionable insights from company data without technical knowledge.",
      examples: [
        "Generate sales reports by region or product category",
        "Track key business metrics and KPIs",
        "Analyze customer segments and behaviors"
      ],
      benefits: [
        "Democratized access to business insights",
        "Faster decision-making based on real-time data",
        "Reduced dependency on data teams for basic reporting"
      ],
      query: "Show me our top 10 customers by revenue this year and how they compare to last year",
      result: "A list of the top 10 customers by revenue this year and how they compare to last year"
    },
  ];

  return (
    <section id="use-cases" className="py-14 sm:py-16 md:py-20 lg:py-24 bg-white relative overflow-hidden">
      <FloatingBackground count={10} opacity={0.02} />
      
      <div className="container mx-auto px-4 sm:px-6 md:px-8 max-w-6xl">
        <div className="text-center mb-10 sm:mb-12 md:mb-16">
          <h2 className="text-3xl sm:text-3xl md:text-4xl font-bold mb-3 md:mb-4">
            NeoBase <span className="text-green-500">Use Cases</span>
          </h2>
          <p className="text-lg sm:text-lg text-gray-700 max-w-3xl mx-auto px-2">
            Discover how NeoBase can transform database interactions for different roles
          </p>
        </div>
        
        {/* Tabs */}
        <div className="flex flex-wrap justify-center gap-2 sm:gap-3 mb-8 sm:mb-10">
          {useCases.map((useCase, index) => (
            <button
              key={index}
              onClick={() => setActiveTab(index)}
              className={`neo-border px-4 py-2 sm:px-5 sm:py-3 flex items-center gap-2 transition-all ${
                activeTab === index 
                  ? 'bg-[#FFDB58] font-bold' 
                  : 'bg-white hover:bg-gray-50'
              }`}
              style={{ transform: `rotate(${Math.random() * 0.6 - 0.3}deg)` }}
            >
              {useCase.icon}
              <span className="text-base sm:text-base">{useCase.title}</span>
            </button>
          ))}
        </div>
        
        {/* Active Tab Content */}
        <div 
          className="neo-border bg-white p-5 sm:p-6 md:p-8"
          style={{ transform: 'rotate(-0.2deg)' }}
        >
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 sm:gap-8">
            <div>
              <h3 className="text-xl sm:text-2xl font-bold mb-4 flex items-center gap-2">
                {useCases[activeTab].icon}
                {useCases[activeTab].title}
              </h3>
              
              <p className="text-base sm:text-lg text-gray-700 mb-6">
                {useCases[activeTab].description}
              </p>
              
              <div className="mb-6">
                <h4 className="font-bold text-lg mb-2">Example Use Cases:</h4>
                <ul className="space-y-2">
                  {useCases[activeTab].examples.map((example, i) => (
                    <li key={i} className="flex items-start gap-2">
                      <span className="bg-[#FFDB58] rounded-full w-5 h-5 flex-shrink-0 flex items-center justify-center text-sm font-bold mt-0.5">
                        {i + 1}
                      </span>
                      <span className="text-base text-gray-700">{example}</span>
                    </li>
                  ))}
                </ul>
              </div>
              
              <div>
                <h4 className="font-bold text-lg mb-2">Key Benefits:</h4>
                <ul className="space-y-2">
                  {useCases[activeTab].benefits.map((benefit, i) => (
                    <li key={i} className="flex items-start gap-2">
                      <span className="text-green-500 flex-shrink-0 mt-0.5">✓</span>
                      <span className="text-base text-gray-700">{benefit}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
            
            <div className="neo-border bg-[#FFDB58]/5 p-4 sm:p-5 md:p-6 flex flex-col">
              <h4 className="font-bold text-lg mb-3">Example Query:</h4>
              
              <div className="flex items-center gap-2 mb-4">
                <div className="w-8 h-8 rounded-full bg-gray-100 flex items-center justify-center">
                  <Users className="w-4 h-4 text-gray-600" />
                </div>
                <div className="neo-border bg-white px-4 py-3 flex-1 text-base italic">
                  "{useCases[activeTab].query}"
                </div>
              </div>
              
              <div className="flex-grow flex flex-col items-center justify-center p-4">
                <LayoutDashboard className="w-12 h-12 text-gray-300 mb-4" />
                <p className="text-center text-gray-500 italic">
                  Result: {useCases[activeTab].result}
                </p>
              </div>
              
              <div className="mt-4 text-center">
                <a 
                  href="https://app.neobase.cloud" 
                  target="_blank" 
                  rel="noopener noreferrer"
                  className="neo-button inline-flex items-center justify-center py-2 px-4 sm:py-3 sm:px-6 font-bold text-base"
                >
                  Try It For Your Use Case
                </a>
              </div>
            </div>
          </div>
        </div>
        <p className='text-center text-gray-500 mt-8 text-lg'>NeoBase fits anywhere & everywhere - <span className='text-black font-medium'>FinTech, E-Commerce, SaaS, Big-Data, etc.</span></p>
      </div>
    </section>
  );
};

export default UseCasesSection; 