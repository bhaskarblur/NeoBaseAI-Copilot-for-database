
import { MessageSquare, Database, LineChart } from 'lucide-react';
import { DefaultToastOptions } from 'react-hot-toast';
import toast from 'react-hot-toast';

const WelcomeSection = ({ isSidebarExpanded, setShowConnectionModal, toastStyle }: { isSidebarExpanded: boolean, setShowConnectionModal: (show: boolean) => void, toastStyle: DefaultToastOptions }) => {
    return (
        <div className={`
            flex-1 
            flex 
            flex-col 
            items-center 
            justify-center
        p-8 
        mt-24
        md:mt-12
        min-h-[calc(100vh-4rem)] 
        transition-all 
        duration-300 
        ${isSidebarExpanded ? 'md:ml-80' : 'md:ml-20'}
    `}>
  {/* Welcome Section */}
  <div className="w-full max-w-4xl mx-auto text-center mb-12">
    <h1 className="text-5xl font-bold mb-4">
     You Ask, NeoBase Will Answer!
    </h1>
    <p className="text-xl text-gray-600 mb-2 max-w-2xl mx-auto">
      Open-source AI Copilot for seamless database interactions.
      <br />
      From SQL to NoSQL, explore and analyze your data through natural conversations.
    </p>
  </div>

  {/* Features Cards */}
  <div className="w-full max-w-4xl mx-auto grid md:grid-cols-3 gap-6 mb-12">
    <button
      onClick={() => {
        toast.success('Talk to your database in plain English. NeoBase translates your questions into database queries automatically.', toastStyle);
      }}
      className="
                    neo-border 
                    bg-white 
                    p-6 
                    rounded-lg
                    text-left
                    transition-all
                    duration-300
                    hover:-translate-y-1
                    hover:shadow-lg
                    hover:bg-[#FFDB58]/5
                    active:translate-y-0
                    disabled:opacity-50
                    disabled:cursor-not-allowed
                "
    >
      <div className="w-12 h-12 bg-[#FFDB58]/20 rounded-lg flex items-center justify-center mb-4">
        <MessageSquare className="w-6 h-6 text-black" />
      </div>
      <h3 className="text-lg font-bold mb-2">
       Ask in Your Language
      </h3>
      <p className="text-gray-600">
        Talk to your database in plain English. NeoBase translates your questions into database queries automatically.
      </p>
    </button>

    <button
      onClick={() => setShowConnectionModal(true)}
      className="
                    neo-border 
                    bg-white 
                    p-6 
                    rounded-lg
                    text-left
                    transition-all
                    duration-300
                    hover:-translate-y-1
                    hover:shadow-lg
                    hover:bg-[#FFDB58]/5
                    active:translate-y-0
                "
    >
      <div className="w-12 h-12 bg-[#FFDB58]/20 rounded-lg flex items-center justify-center mb-4">
        <Database className="w-6 h-6 text-black" />
      </div>
      <h3 className="text-lg font-bold mb-2">
        Multi-Database Support
      </h3>
      <p className="text-gray-600">
        Connect to PostgreSQL, MySQL, MongoDB, Redis, and more. One interface for all your databases.
      </p>
    </button>

    <button
      onClick={() => {
        toast.success('Your data is visualized in tables or JSON format. Execute queries and see results in real-time.', toastStyle);
      }}
      className="
                    neo-border 
                    bg-white 
                    p-6 
                    rounded-lg
                    text-left
                    transition-all
                    duration-300
                    hover:-translate-y-1
                    hover:shadow-lg
                    hover:bg-[#FFDB58]/5
                    active:translate-y-0
                    disabled:opacity-50
                    disabled:cursor-not-allowed
                "
    >
      <div className="w-12 h-12 bg-[#FFDB58]/20 rounded-lg flex items-center justify-center mb-4">
        <LineChart className="w-6 h-6 text-black" />
      </div>
      <h3 className="text-lg font-bold mb-2">
        Visualize Results
      </h3>
      <p className="text-gray-600">
        View your data in tables or JSON format. Execute queries &amp; see results in real-time. Charts coming soon.
      </p>
    </button>
  </div>

  {/* CTA Section */}
  <div className="text-center">
    <button
      onClick={() => setShowConnectionModal(true)}
      className="neo-button text-lg px-8 py-4 mb-4"
    >
      Create New Connection
    </button>
    <p className="text-gray-600">
      or select an existing one from the sidebar to begin
    </p>
  </div>
        </div>
    )
}

export default WelcomeSection;