interface DatabaseLogoProps {
  type: 'postgresql' | 'yugabytedb' | 'mysql' | 'mongodb' | 'redis' | 'clickhouse' | 'neo4j' | 'cassandra' | 'spreadsheet' | 'google_sheets';
  size?: number;
  className?: string;
}

// Import all logos using Vite's import.meta.env.BASE_URL
const databaseLogos: Record<Exclude<DatabaseLogoProps['type'], 'spreadsheet' | 'google_sheets'>, string> = {
  postgresql: `${import.meta.env.VITE_FRONTEND_BASE_URL}postgresql-logo.png`,
  yugabytedb: `${import.meta.env.VITE_FRONTEND_BASE_URL}yugabytedb-logo.svg`,
  mysql: `${import.meta.env.VITE_FRONTEND_BASE_URL}mysql-logo.png`,
  mongodb: `${import.meta.env.VITE_FRONTEND_BASE_URL}mongodb-logo.svg`,
  redis: `${import.meta.env.VITE_FRONTEND_BASE_URL}redis-logo.svg`,
  clickhouse: `${import.meta.env.VITE_FRONTEND_BASE_URL}clickhouse-logo.svg`,
  neo4j: `${import.meta.env.VITE_FRONTEND_BASE_URL}neo4j-logo.png`,
  cassandra: `${import.meta.env.VITE_FRONTEND_BASE_URL}cassandra-logo.png`
};

export default function DatabaseLogo({ type, size = 24, className = '' }: DatabaseLogoProps) {
  // Special handling for Google Sheets to show the Google Sheets logo
  if (type === 'google_sheets') {
    return (
      <div
        className={`relative flex items-center justify-center ${className}`}
        style={{ width: size, height: size }}
      >
        <img
          src={`${import.meta.env.VITE_FRONTEND_BASE_URL}gsheets-logo.png`}
          alt="Google Sheets logo"
          className="w-full h-full object-contain"
          onError={(e) => {
            console.error('Google Sheets logo failed to load');
            // Fallback to a spreadsheet icon if Google Sheets logo fails
            e.currentTarget.style.display = 'none';
            const parent = e.currentTarget.parentElement;
            if (parent) {
              parent.innerHTML = `<svg
                viewBox="0 0 24 24"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
                className="w-full h-full"
              >
                <rect x="3" y="3" width="18" height="18" rx="2" stroke="#34a853" strokeWidth="2"/>
                <line x1="3" y1="9" x2="21" y2="9" stroke="#34a853" strokeWidth="2"/>
                <line x1="3" y1="15" x2="21" y2="15" stroke="#34a853" strokeWidth="2"/>
                <line x1="9" y1="3" x2="9" y2="21" stroke="#34a853" strokeWidth="2"/>
                <line x1="15" y1="3" x2="15" y2="21" stroke="#34a853" strokeWidth="2"/>
              </svg>`;
            }
          }}
        />
      </div>
    );
  }

  // Special handling for spreadsheet type to show a custom icon
  if (type === 'spreadsheet') {
    return (
      <div
        className={`relative flex items-center justify-center ${className}`}
        style={{ width: size, height: size }}
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
          className="w-full h-full"
        >
          <rect x="3" y="3" width="18" height="18" rx="2" stroke="#10B981" strokeWidth="2"/>
          <line x1="3" y1="9" x2="21" y2="9" stroke="#10B981" strokeWidth="2"/>
          <line x1="3" y1="15" x2="21" y2="15" stroke="#10B981" strokeWidth="2"/>
          <line x1="9" y1="3" x2="9" y2="21" stroke="#10B981" strokeWidth="2"/>
          <line x1="15" y1="3" x2="15" y2="21" stroke="#10B981" strokeWidth="2"/>
        </svg>
      </div>
    );
  }

  return (
    <div
      className={`relative flex items-center justify-center ${className}`}
      style={{ width: size, height: size }}
    >
      <img
        src={databaseLogos[type]}
        alt={`${type} database logo`}
        className="w-full h-full object-contain"
        onError={(e) => {
          console.error('Logo failed to load:', {
            type,
            src: e.currentTarget.src,
            error: e
          });
          // Fallback to a generic database icon if the logo fails to load
          e.currentTarget.style.display = 'none';
          const parent = e.currentTarget.parentElement;
          if (parent) {
            parent.innerHTML = `<svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              className="w-full h-full"
            >
              <path d="M4 7c0-1.1.9-2 2-2h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7z" />
              <path d="M4 7h16" />
              <path d="M4 11h16" />
              <path d="M4 15h16" />
            </svg>`;
          }
        }}
      />
    </div>
  );
}