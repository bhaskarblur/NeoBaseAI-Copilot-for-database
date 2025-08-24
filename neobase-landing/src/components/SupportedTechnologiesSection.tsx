import React from 'react';
import FloatingBackground from './FloatingBackground';

interface Logo {
  name: string;
  src: string;
  alt: string;
}

const SupportedTechnologiesSection: React.FC = () => {
  const allLogos: Logo[] = [
    { name: 'PostgreSQL', src: '/postgresql-logo.png', alt: 'PostgreSQL Logo' },
    { name: 'MongoDB', src: '/mongodb-logo.svg', alt: 'MongoDB Logo' },
    { name: 'MySQL', src: '/mysql-logo.png', alt: 'MySQL Logo' },
    { name: 'Redis', src: '/redis-logo.svg', alt: 'Redis Logo' },
    { name: 'Cassandra', src: '/cassandra-logo.png', alt: 'Cassandra Logo' },
    { name: 'ClickHouse', src: '/clickhouse-logo.svg', alt: 'ClickHouse Logo' },
    { name: 'YugabyteDB', src: '/yugabytedb-logo.svg', alt: 'YugabyteDB Logo' },
    { name: 'Neo4j', src: '/neo4j-logo.png', alt: 'Neo4j Logo' },
    { name: 'Google Sheets', src: '/gsheets-logo.png', alt: 'Google Sheets Logo' },
    { name: 'Microsoft Excel', src: '/excel-logo.png', alt: 'Microsoft Excel Logo' },
    { name: 'SQLite', src: '/sqlite-logo.png', alt: 'SQLite Logo' },
    { name: 'CockroachDB', src: '/cockroachdb-logo.webp', alt: 'CockroachDB Logo' },
    {name: "MariaDB", src: "/mariadb-logo.png", alt: "MariaDB Logo"},
  ];

  // Create seamless infinite scroll - ensure all logos show
  const duplicatedLogos = [...allLogos, ...allLogos];
  
  console.log('Total logos:', allLogos.length);
  console.log('All logo names:', allLogos.map(logo => logo.name));

  return (
    <section id="technologies" className="py-12 sm:py-16 md:py-20 lg:py-24 bg-white relative overflow-hidden">
      <FloatingBackground count={18} opacity={0.03} />
      
      <div className="container mx-auto px-4 sm:px-6 md:px-8 max-w-7xl">
        <div className="text-center mb-8 sm:mb-12 md:mb-16">
          <h2 className="text-3xl sm:text-3xl md:text-4xl font-bold mb-3 md:mb-4">
            <span className="text-green-500">Wide range</span> of Integrations
          </h2>
          <p className="text-lg sm:text-lg text-gray-700 max-w-3xl mx-auto px-2">
            NeoBase works with a variety of data sources and technologies, with more being added regularly.
          </p>
        </div>
        
        {/* Horizontal Marquee */}
        <div className="relative overflow-hidden">
          {/* Gradient fade edges */}
          <div className="absolute left-0 top-0 bottom-0 w-20 bg-gradient-to-r from-white to-transparent z-10"></div>
          <div className="absolute right-0 top-0 bottom-0 w-20 bg-gradient-to-l from-white to-transparent z-10"></div>
          
          {/* Marquee container */}
          <div className="flex animate-marquee hover:pause-marquee" style={{ width: 'calc(200% + 100px)' }}>
            {duplicatedLogos.map((logo, index) => (
              <div key={`${logo.name}-${index}`} className="flex-shrink-0 mx-4 sm:mx-6">
                <div className="flex flex-col items-center group cursor-pointer">
                  <div className="bg-white neo-border p-4 sm:p-6 rounded-lg transition-all duration-300 hover:shadow-lg">
                    <img 
                      src={logo.src} 
                      alt={logo.alt}
                      className="w-14 h-14 object-contain transition-all duration-300"
                    />
                  </div>
                  <span className="text-xs sm:text-sm font-medium text-gray-600 mt-2 opacity-0 group-hover:opacity-100 transition-opacity duration-300">
                    {logo.name}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="mt-8 sm:mt-12 text-center">
          <p className="text-base sm:text-base text-gray-600 italic">
            Don't see your data source? <a href="https://github.com/bhaskarblur/neobase-ai-dba/issues" className="text-green-600 hover:text-green-700 underline font-medium">Raise a request</a>
          </p>
        </div>
      </div>

      <style dangerouslySetInnerHTML={{
        __html: `
          @keyframes marquee {
            0% {
              transform: translateX(0%);
            }
            100% {
              transform: translateX(-50%);
            }
          }
          
          .animate-marquee {
            animation: marquee 8s linear infinite;
          }
          
          @media (min-width: 640px) {
            .animate-marquee {
              animation: marquee 12s linear infinite;
            }
          }
          
          @media (min-width: 1024px) {
            .animate-marquee {
              animation: marquee 16s linear infinite;
            }
          }
          
          .pause-marquee {
            animation-play-state: paused;
          }
        `
      }} />
    </section>
  );
};

export default SupportedTechnologiesSection; 