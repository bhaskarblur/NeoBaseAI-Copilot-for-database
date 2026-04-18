import React, { useState } from 'react';
import { ChevronDown } from 'lucide-react';
import FloatingBackground from './FloatingBackground';

interface FAQItem {
  question: string;
  answer: React.ReactNode;
}

const FAQSection: React.FC = () => {
  const [openIndex, setOpenIndex] = useState<number | null>(null);

  const toggleFAQ = (index: number) => {
    setOpenIndex(openIndex === index ? null : index);
  };

  const faqs: FAQItem[] = [
    {
      question: "What is NeoBase & Why made it ?",
      answer: (
        <p>
          NeoBase is an AI Database Co-Pilot that connects you to your data in your daily language such as English or Hindi.
          This means that you can ask questions in your own language and NeoBase will do the rest.
          You do not need to know SQL or any other database query language to use NeoBase, just ask in your own language.
          <br />
          <br />
          NeoBase was built to enable both Technical & Non-Technical people of the team to visualize the data they are dealing with, with ease, in their own language in real-time.
          Our vision is to become the `Database Orchestrator` for all your data needs.
        </p>
      )
    },
    {
      question: "Which databases does NeoBase support?",
      answer: (
        <div>
          <p className="mb-2">NeoBase currently supports the following databases:</p>
          <ul className="list-disc pl-6 space-y-1">
            <li>PostgreSQL</li>
            <li>Yugabyte</li>
            <li>MySQL</li>
            <li>ClickHouse</li>
            <li>MongoDB</li>
          </ul>
          <p className="mt-2">
            Cassandra, Redis, and Neo4j and many more databases are planned for future releases.
          </p>
        </div>
      )
    },
    {
      question: "Which LLM clients does NeoBase support?",
      answer: (
        <div>
          <p className="mb-2">NeoBase currently supports the following LLM clients:</p>
          <ul className="list-disc pl-6 space-y-1">
            <li>OpenAI (Any chat completion model)</li>
            <li>Google Gemini (Any chat completion model)</li>
          </ul>
          <p className="mt-2">
            Support for Anthropic (Sonnet) and Ollama is planned for future releases.
          </p>
        </div>
      )
    },
    {
        question: "How does NeoBase ensure security of database credentials & data?",
        answer: (
          <p>
            NeoBase uses industry-standard encryption and secure protocols to protect your database credentials & data. 
            All data is stored in your own database, and it does not store any data on its servers.
            NeoBase doesn't send your query results to any LLM clients, it runs the query on your database and returns the results only to you.
            <br />
            <br />
            PS: <span className="font-bold">Since NeoBase is Open source & allow self-hosted usage</span>, you are in full control of your data.
          </p>
        )
      },

    {
        question: "Is NeoBase open source?",
        answer: (
          <p>
            Yes, NeoBase is fully open source and self-hosted. You can deploy it on your own 
            infrastructure with full control. The project is licensed under the MIT License.
          </p>
        )
      },
    {
      question: "How do I set up NeoBase on my own infrastructure?",
      answer: (
        <div>
          <p className="mb-2">To set up NeoBase:</p>
          <ol className="list-decimal pl-6 space-y-1">
            <li>Follow the instructions in the SETUP.md file in the GitHub repository.</li>
            <li>Create a new user in the app using admin credentials.</li>
            <li>Generate a user signup secret via admin credentials.  (Only required if you are using `ENVIRONMENT` as `production`)</li>
            <li>Use this secret to sign up a new user from the NeoBase UI. (Only required if you are using `ENVIRONMENT` as `production`)</li>
          </ol>
          <p className="mt-2">
            For detailed setup instructions, please refer to the <a href="https://github.com/bhaskarblur/neobase-ai-dba/blob/main/SETUP.md" className="text-green-600 hover:text-green-700 underline font-medium">SETUP.md</a> file.
          </p>
        </div>
      )
    },
  ];

  return (
    <section id="faq" className="py-14 sm:py-16 md:py-20 lg:pt-24 bg-white relative overflow-hidden">
      <FloatingBackground count={12} opacity={0.02} />
      
      <div className="container mx-auto px-4 sm:px-6 md:px-8 max-w-6xl">
        <div className="text-center mb-10 sm:mb-12 md:mb-16">
          <h2 className="text-3xl sm:text-3xl md:text-4xl font-bold mb-3 md:mb-4">
            Frequently Asked <span className="text-green-500">Questions</span>
          </h2>
          <p className="text-lg sm:text-lg text-gray-700 max-w-3xl mx-auto px-2">
            Everything you need to know about NeoBase
          </p>
        </div>
        
        <div className="space-y-5 sm:space-y-6 md:space-y-8">
          {faqs.map((faq, index) => (
            <div 
              key={index}
              className="neo-border overflow-hidden transition-all duration-300 w-full"
              style={{ transform: `rotate(${index % 2 === 0 ? '-0.3deg' : '0.3deg'})` }}
            >
              <button
                className="w-full p-5 sm:p-6 text-left flex justify-between items-center bg-white hover:bg-gray-50 transition-colors duration-200"
                onClick={() => toggleFAQ(index)}
                aria-expanded={openIndex === index}
              >
                <h3 className="font-bold text-lg sm:text-xl">{faq.question}</h3>
                <ChevronDown 
                  className={`w-5 h-5 sm:w-6 sm:h-6 flex-shrink-0 transition-transform duration-300 ${openIndex === index ? 'rotate-180' : ''}`} 
                />
              </button>
              <div 
                className={`overflow-hidden transition-all duration-300 ${
                  openIndex === index ? 'max-h-[500px] opacity-100' : 'max-h-0 opacity-0'
                }`}
              >
                <div className="p-5 sm:p-6 pt-0 sm:pt-0 bg-[#FFDB58]/5 text-base sm:text-lg text-gray-700">
                  {faq.answer}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default FAQSection; 