import React from 'react';
import { Check, X } from 'lucide-react';
import FloatingBackground from './FloatingBackground';

interface ComparisonItem {
  feature: string;
  neobase: boolean;
  traditional: boolean;
  highlight?: boolean;
}

const ComparisonSection: React.FC = () => {
  const comparisonItems: ComparisonItem[] = [
    { 
      feature: "Natural Language Queries", 
      neobase: true, 
      traditional: false,
      highlight: true
    },
    { 
      feature: "Multi-Source Support", 
      neobase: true, 
      traditional: false 
    },
    { 
      feature: "Complex & Custom Data Fetching", 
      neobase: true, 
      traditional: false,
      highlight: true
    },
    { 
      feature: "Real-time Results", 
      neobase: true, 
      traditional: true,
      highlight: false
    },
    { 
      feature: "Flexible for different applications", 
      neobase: true, 
      traditional: false 
    },
    { 
      feature: "Data Visualization", 
      neobase: true, 
      traditional: true,
      highlight: true
    },
    { 
      feature: "Query Optimization", 
      neobase: true, 
      traditional: false 
    },
  ];

  return (
    <section id="comparison" className="py-14 sm:py-16 md:py-20 lg:py-24 bg-[#FFDB58]/10 relative overflow-hidden">
      <FloatingBackground count={15} opacity={0.05} />
      
      <div className="container mx-auto px-4 sm:px-6 md:px-8 max-w-6xl">
        <div className="text-center mb-10 sm:mb-12 md:mb-16">
          <h2 className="text-3xl sm:text-3xl md:text-4xl font-bold mb-3 md:mb-4">
            Why Choose <span className="text-yellow-500">NeoBase</span>
          </h2>
          <p className="text-lg sm:text-lg text-gray-700 max-w-3xl mx-auto px-2">
            See how NeoBase transforms your database experience compared to traditional methods
          </p>
        </div>
        
        <div className="neo-border bg-white overflow-hidden">
          {/* Header */}
          <div className="grid grid-cols-3 text-center font-bold border-b border-black">
            <div className="p-4 sm:p-5 border-r border-black">Features</div>
            <div className="p-4 sm:p-5 bg-[#FFDB58]/20 border-r border-black">NeoBase - AI Database Co-Pilot</div>
            <div className="p-4 sm:p-5">Traditional Dashboards, Admin Panels</div>
          </div>

          {/* Comparison Items */}
          {comparisonItems.map((item, index) => (
            <div 
              key={index} 
              className={`grid grid-cols-3 text-center border-b border-black last:border-b-0 ${item.highlight ? 'bg-[#FFDB58]/5' : ''}`}
            >
              <div className="p-4 sm:p-5 border-r border-black text-left font-medium">
                {item.feature}
              </div>
              <div className="p-4 sm:p-5 border-r border-black flex justify-center items-center">
                {item.neobase ? (
                  <Check size={24} className="text-green-500" />
                ) : (
                  <X size={24} className="text-red-500" />
                )}
              </div>
              <div className="p-4 sm:p-5 flex justify-center items-center">
                {item.traditional ? (
                  <Check size={24} className="text-green-500" />
                ) : (
                  <X size={24} className="text-red-500" />
                )}
              </div>
            </div>
          ))}
        </div>

      </div>
    </section>
  );
};

export default ComparisonSection; 