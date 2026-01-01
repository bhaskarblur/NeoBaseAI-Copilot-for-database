import React from 'react';

interface SettingsTabProps {
  autoExecuteQuery: boolean;
  shareWithAI: boolean;
  nonTechMode: boolean;
  autoGenerateVisualization: boolean;
  setAutoExecuteQuery: (value: boolean) => void;
  setShareWithAI: (value: boolean) => void;
  setNonTechMode: (value: boolean) => void;
  setAutoGenerateVisualization: (value: boolean) => void;
}

const SettingsTab: React.FC<SettingsTabProps> = ({
  autoExecuteQuery,
  shareWithAI,
  nonTechMode,
  autoGenerateVisualization,
  setAutoExecuteQuery,
  setShareWithAI,
  setNonTechMode,
  setAutoGenerateVisualization,
}) => {
  return (
    <div className="space-y-6">
      <div className="neo-border p-4 rounded-lg">
        <div className="flex flex-col md:flex-row gap-4 md:gap-0 items-start md:items-center justify-between">
          <div>
            <label className="block font-bold mb-1 text-lg">Auto Fetch Results</label>
            <p className="text-gray-600 text-sm max-w-[480px]">Automatically fetches results of the AI operation from the database upon the AI response. However, the critical operations such as Updating, Inserting, Deleting, etc. still need to be executed manually by the user.</p>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input 
              type="checkbox" 
              className="sr-only peer" 
              checked={autoExecuteQuery}
              onChange={(e) => {
                const newValue = e.target.checked;
                setAutoExecuteQuery(newValue);
              }}
            />
            <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        </div>
      </div>

      <div className="neo-border p-4 rounded-lg">
        <div className="flex flex-col md:flex-row gap-4 md:gap-0 items-start md:items-center justify-between">
          <div>
            <label className="block font-bold mb-1 text-lg">Share Result With AI</label>
            <p className="text-gray-600 text-sm max-w-[480px]">Allow NeoBase to share your query results with AI for better responses. This can be useful for use-cases such as asking queries from your previous messages results. <br/>Note: This will take more tokens and size that is being sent to the AI.</p>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input 
              type="checkbox" 
              className="sr-only peer" 
              checked={shareWithAI}
              onChange={(e) => {
                setShareWithAI(e.target.checked);
              }}
            />
            <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        </div>
      </div>

      <div className="neo-border p-4 rounded-lg">
        <div className="flex flex-col md:flex-row gap-4 md:gap-0 items-start md:items-center justify-between">
          <div>
            <label className="block font-bold mb-1 text-lg">Non-Technical Mode</label>
            <p className="text-gray-600 text-sm max-w-[480px]">Enable simplified, non-technical explanations and results. AI will provide easy-to-understand responses, hide complex queries, and show only relevant data fields. <strong>Ideal for business users</strong> who want insights without technical details.<br />Note: This setting will not be applied to the existing messages.</p>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input 
              type="checkbox" 
              className="sr-only peer" 
              checked={nonTechMode}
              onChange={(e) => {
                setNonTechMode(e.target.checked);
              }}
            />
            <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        </div>
      </div>

      <div className="neo-border p-4 rounded-lg">
        <div className="flex flex-col md:flex-row gap-4 md:gap-0 items-start md:items-center justify-between">
          <div>
            <label className="block font-bold mb-1 text-lg">Auto Generate Visualizations</label>
            <p className="text-gray-600 text-sm max-w-[480px]">Automatically generate visualizations for query results that can be meaningfully visualized. The AI will analyze your query results and create appropriate views (line, charts, bar, pie, scatter, area). <strong>Enhances data insights</strong> with automatic visual representations.<br />Note: Visualization generation adds some processing time to AI responses.</p>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input 
              type="checkbox" 
              className="sr-only peer" 
              checked={autoGenerateVisualization}
              onChange={(e) => {
                setAutoGenerateVisualization(e.target.checked);
              }}
            />
            <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        </div>
      </div>

      <div className="p-4 bg-yellow-100 border border-yellow-200 rounded-lg">
        <p className="text-sm text-yellow-700 font-medium">
          More settings coming soon! We're constantly working to improve the configuration options for your database connections.
        </p>
      </div>
    </div>
  );
};

export default SettingsTab; 