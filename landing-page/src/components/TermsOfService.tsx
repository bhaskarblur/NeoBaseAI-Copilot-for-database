import { Link } from 'react-router-dom';
import { ArrowLeft, FileText, Database, Users, AlertTriangle, Scale, Shield, CheckCircle, XCircle, Building, Mail, Phone, MapPin, Globe } from 'lucide-react';
import Navbar from './Navbar';
import Footer from './Footer';
import FloatingBackground from './FloatingBackground';
import { useGitHubStats } from '../hooks/useGitHubStats';

const TermsOfService = () => {
  const { forks } = useGitHubStats();
  return (
    <div className="min-h-screen bg-[#FFDB58]/10">
      <Navbar forks={forks} />
      
      {/* Main Content */}
      <main className="relative overflow-hidden">
        <FloatingBackground count={8} opacity={0.1} />
        
        <div className="container mx-auto px-6 md:px-8 lg:px-0 max-w-7xl py-12">
          {/* Back Button */}
          <Link 
            to="/"
            className="neo-button-secondary inline-flex items-center gap-2 mb-8"
          >
            <ArrowLeft className="w-4 h-4" />
            Back to Home
          </Link>

          {/* Header */}
          <div className="text-center mb-12">
            <div className="inline-flex items-center gap-4 mb-6">
              <div className="neo-border bg-black text-white p-4">
                <FileText className="w-8 h-8" />
              </div>
            </div>
            <h1 className="text-4xl md:text-4xl font-bold mb-4">Terms of Service</h1>
            <p className="text-base text-gray-600">Last updated: August 24, 2025</p>
          </div>

          <div className="space-y-12">
            {/* Introduction */}
            <section className="neo-border bg-[#FFDB58]/10 p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <CheckCircle className="w-8 h-8" />
                Agreement to Terms
              </h2>
              <p className="text-lg leading-relaxed mb-4">
                Welcome to NeoBase! These Terms of Service ("Terms") govern your use of NeoBase, 
                an AI Data Copilot developed and operated by Aurivance Technologies LLP ("Company," "we," "us," or "our"). 
                By accessing or using our service, you agree to be bound by these Terms. 
                If you disagree with any part of these terms, then you may not access the service.
              </p>
              <div className="neo-border bg-white p-4">
                <p className="text-base leading-relaxed font-medium">
                  <strong>Important:</strong> These Terms constitute a legally binding agreement between you and Aurivance Technologies LLP. 
                  Please read them carefully and keep a copy for your records.
                </p>
              </div>
            </section>

            {/* Company Information */}
            <section className="neo-border bg-gray-50 p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <Building className="w-8 h-8 text-gray-600" />
                About Aurivance Technologies LLP
              </h2>
              <p className="text-lg leading-relaxed mb-6">
                NeoBase is developed and operated by Aurivance Technologies LLP, a technology company incorporated in India, specializing in AI-powered data solutions and enterprise software development.
              </p>
              <div className="grid md:grid-cols-2 gap-6">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4 text-gray-800">Legal Entity</h3>
                  <ul className="space-y-2 text-gray-700">
                    <li><strong>Company Name:</strong> Aurivance Technologies LLP</li>
                    <li><strong>Legal Structure:</strong> Limited Liability Partnership</li>
                    <li><strong>Registered in:</strong> India</li>
                    <li><strong>Business Domain:</strong> AI & Data Technology Solutions</li>
                  </ul>
                </div>
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4 text-gray-800">Service Focus</h3>
                  <ul className="space-y-2 text-gray-700">
                    <li>AI-powered database assistance</li>
                    <li>Enterprise data management</li>
                    <li>Self-hosted solutions</li>
                    <li>Open-source technology</li>
                  </ul>
                </div>
              </div>
            </section>

            {/* Service Description */}
            <section className="neo-border bg-gray-50 p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <Database className="w-8 h-8" />
                Service Description
              </h2>
              
              <p className="text-lg leading-relaxed mb-6">
                NeoBase is an AI Data Copilot that provides comprehensive database management and analysis capabilities:
              </p>
              <div className="grid md:grid-cols-2 gap-6">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">Core Features</h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>Natural Language Processing:</strong> Convert plain English to database queries</li>
                    <li><strong>Multi-Database Support:</strong> PostgreSQL, MySQL, MongoDB, ClickHouse, and more</li>
                    <li><strong>Spreadsheet Integration:</strong> CSV, Excel, and Google Sheets support</li>
                    <li><strong>AI-Powered Analysis:</strong> Intelligent data insights and recommendations</li>
                    <li><strong>Real-time Collaboration:</strong> Chat-based interface with AI assistant</li>
                    <li><strong>Query Optimization:</strong> Performance suggestions and best practices</li>
                  </ul>
                </div>
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">Deployment Options</h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>Cloud Hosting:</strong> Managed service with high availability</li>
                    <li><strong>Self-Hosted:</strong> Deploy on your own infrastructure</li>
                    <li><strong>On-Premises:</strong> Complete control over data and privacy</li>
                    <li><strong>Hybrid Solutions:</strong> Mix of cloud and on-premises components</li>
                    <li><strong>Open Source:</strong> MIT license for community contributions</li>
                    <li><strong>Enterprise Support:</strong> Custom solutions and dedicated support</li>
                  </ul>
                </div>
              </div>
            </section>

            {/* User Responsibilities */}
            <section className="neo-border bg-white p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <Users className="w-8 h-8" />
                User Responsibilities & Obligations
              </h2>
              
              <div className="space-y-6">
                <div className="grid md:grid-cols-2 gap-6">
                  <div className="neo-border bg-white p-6">
                    <h3 className="text-xl font-bold mb-4 flex items-center gap-2">
                      <CheckCircle className="w-6 h-6" />
                      You Must:
                    </h3>
                    <ul className="list-disc list-inside space-y-2 text-gray-700">
                      <li><strong>Provide accurate information</strong> when creating accounts and connections</li>
                      <li><strong>Maintain secure credentials</strong> and protect your login information</li>
                      <li><strong>Use the service legally</strong> in compliance with applicable laws and regulations</li>
                      <li><strong>Respect intellectual property</strong> rights of others and our platform</li>
                      <li><strong>Only access authorized databases</strong> that you own or have permission to use</li>
                      <li><strong>Report security issues</strong> responsibly through proper channels</li>
                      <li><strong>Keep software updated</strong> (for self-hosted deployments)</li>
                      <li><strong>Follow best practices</strong> for database security and access control</li>
                    </ul>
                  </div>
                  <div className="neo-border bg-white p-6">
                    <h3 className="text-xl font-bold mb-4 flex items-center gap-2">
                      <XCircle className="w-6 h-6" />
                      You Must Not:
                    </h3>
                    <ul className="list-disc list-inside space-y-2 text-gray-700">
                      <li><strong>Use for illegal purposes</strong> or violate any applicable laws</li>
                      <li><strong>Access unauthorized data</strong> or attempt to breach other users' accounts</li>
                      <li><strong>Reverse engineer</strong> or attempt to extract proprietary algorithms</li>
                      <li><strong>Interfere with service operation</strong> or attempt to disrupt systems</li>
                      <li><strong>Use automated tools</strong> to scrape or abuse the platform</li>
                      <li><strong>Share account credentials</strong> or allow unauthorized access</li>
                      <li><strong>Violate third-party rights</strong> or infringe on intellectual property</li>
                      <li><strong>Transmit malicious code</strong> or attempt to harm the platform</li>
                    </ul>
                  </div>
                </div>

                <div className="neo-border bg-[#FFDB58]/10 p-6">
                  <h3 className="text-xl font-bold mb-4">Professional Use Guidelines</h3>
                  <div className="grid md:grid-cols-3 gap-6">
                    <div>
                      <h4 className="font-bold mb-3">Database Access</h4>
                      <ul className="list-disc list-inside space-y-1 text-gray-700">
                        <li>Verify authorization before connecting</li>
                        <li>Use read-only access when possible</li>
                        <li>Follow organizational data policies</li>
                        <li>Implement proper backup procedures</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Data Handling</h4>
                      <ul className="list-disc list-inside space-y-1 text-gray-700">
                        <li>Classify data sensitivity levels</li>
                        <li>Apply appropriate encryption</li>
                        <li>Regular security reviews</li>
                        <li>Incident response procedures</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Compliance</h4>
                      <ul className="list-disc list-inside space-y-1 text-gray-700">
                        <li>Meet regulatory requirements</li>
                        <li>Document access and usage</li>
                        <li>Regular compliance audits</li>
                        <li>Staff training and awareness</li>
                      </ul>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            {/* Data and Privacy */}
            <section>
              <h2 className="text-2xl font-bold mb-4 flex items-center gap-3">
                <Shield className="w-6 h-6" />
                Data Ownership & Privacy
              </h2>
              
              <div className="space-y-6">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">Your Data Rights</h3>
                  <div className="grid md:grid-cols-2 gap-6">
                    <div>
                      <h4 className="font-bold mb-3">Complete Ownership</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li><strong>You retain full ownership</strong> of all data you input into NeoBase</li>
                        <li><strong>Your database content remains yours</strong> - we claim no rights to it</li>
                        <li><strong>Your queries and results</strong> belong to you</li>
                        <li><strong>Chat history and interactions</strong> are your intellectual property</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Data Control</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li><strong>Choose what to share</strong> with AI services through privacy settings</li>
                        <li><strong>Export your data</strong> at any time in standard formats</li>
                        <li><strong>Delete data selectively</strong> or completely</li>
                        <li><strong>Set retention policies</strong> according to your needs</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="neo-border bg-gray-50 p-6">
                  <h3 className="text-xl font-bold mb-4">Our Data Commitments</h3>
                  <div className="grid md:grid-cols-2 gap-6">
                    <div>
                      <h4 className="font-bold mb-3">What We Don't Do</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>❌ <strong>Never sell or rent</strong> your data to third parties</li>
                        <li>❌ <strong>Never use your database content</strong> to train our AI models</li>
                        <li>❌ <strong>Never share sensitive data</strong> without explicit consent</li>
                        <li>❌ <strong>Never access your databases</strong> without your permission</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">What We Do</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>✅ <strong>Encrypt all data</strong> using industry-standard methods</li>
                        <li>✅ <strong>Provide transparency</strong> about data usage and sharing</li>
                        <li>✅ <strong>Enable self-hosting</strong> for complete data control</li>
                        <li>✅ <strong>Implement strong security</strong> measures and access controls</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="neo-border bg-[#FFDB58]/10 p-6">
                  <h3 className="text-xl font-bold mb-4">Self-Hosting Benefits</h3>
                  <p className="text-lg leading-relaxed mb-4">
                    For maximum privacy and control, we strongly recommend self-hosted deployments:
                  </p>
                  <div className="grid md:grid-cols-3 gap-4">
                    <div className="neo-border bg-white p-4">
                      <h4 className="font-bold mb-3">Complete Control</h4>
                      <ul className="list-disc list-inside space-y-1 text-xs text-gray-600">
                        <li>Data never leaves your infrastructure</li>
                        <li>Your own encryption keys</li>
                        <li>Custom security policies</li>
                        <li>Full audit capabilities</li>
                      </ul>
                    </div>
                    <div className="neo-border bg-white p-4">
                      <h4 className="font-bold mb-3">Compliance</h4>
                      <ul className="list-disc list-inside space-y-1 text-xs text-gray-600">
                        <li>Meet regulatory requirements</li>
                        <li>Data residency compliance</li>
                        <li>Custom retention policies</li>
                        <li>Industry-specific controls</li>
                      </ul>
                    </div>
                    <div className="neo-border bg-white p-4">
                      <h4 className="font-bold mb-3">Customization</h4>
                      <ul className="list-disc list-inside space-y-1 text-xs text-gray-600">
                        <li>Modify source code</li>
                        <li>Custom integrations</li>
                        <li>Branded deployment</li>
                        <li>Enterprise features</li>
                      </ul>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            {/* Acceptable Use Policy */}
            <section>
              <h2 className="text-2xl font-bold mb-4 flex items-center gap-3">
                <Scale className="w-6 h-6" />
                Acceptable Use Policy
              </h2>
              
              <div className="space-y-6">
                <div className="neo-border bg-gray-50 p-6">
                  <h3 className="text-xl font-bold mb-4">Database Connections & Access</h3>
                  <div className="grid md:grid-cols-2 gap-6">
                    <div>
                      <h4 className="font-bold mb-3">Authorization Requirements</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>Only connect to databases you <strong>own or have explicit permission</strong> to access</li>
                        <li>Ensure <strong>proper authorization</strong> before querying production databases</li>
                        <li>Follow your <strong>organization's database policies</strong> and procedures</li>
                        <li>Use <strong>read-only connections</strong> when possible to minimize risk</li>
                        <li>Implement <strong>least-privilege access</strong> principles</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Security Best Practices</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>Use <strong>strong, unique passwords</strong> for database connections</li>
                        <li>Enable <strong>SSL/TLS encryption</strong> for all database connections</li>
                        <li>Regularly <strong>rotate credentials</strong> and access tokens</li>
                        <li>Monitor <strong>access logs and audit trails</strong></li>
                        <li>Report <strong>suspicious activities</strong> immediately</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">AI Usage Guidelines</h3>
                  <div className="grid md:grid-cols-2 gap-6">
                    <div>
                      <h4 className="font-bold mb-3">Responsible AI Use</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>Use AI features for <strong>legitimate database management</strong> purposes</li>
                        <li><strong>Review AI-generated queries</strong> before execution, especially on production</li>
                        <li><strong>Don't rely solely on AI</strong> for critical database operations</li>
                        <li>Understand that <strong>AI responses may not always be perfect</strong></li>
                        <li>Provide <strong>clear context and requirements</strong> in your queries</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Data Sensitivity</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>Be mindful of <strong>sensitive data</strong> in your queries</li>
                        <li>Use <strong>privacy settings</strong> to control AI data sharing</li>
                        <li>Consider <strong>data masking</strong> for development/testing</li>
                        <li>Follow <strong>data classification</strong> policies</li>
                        <li>Implement <strong>data loss prevention</strong> measures</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="neo-border bg-gray-50 p-6">
                  <h3 className="text-xl font-bold mb-4">Resource Usage & Performance</h3>
                  <div className="grid md:grid-cols-3 gap-6">
                    <div>
                      <h4 className="font-bold mb-3">Fair Use</h4>
                      <ul className="list-disc list-inside space-y-1 text-sm text-gray-700">
                        <li>Use the service reasonably</li>
                        <li>Don't abuse system resources</li>
                        <li>Avoid excessive API calls</li>
                        <li>Monitor your usage patterns</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Performance</h4>
                      <ul className="list-disc list-inside space-y-1 text-sm text-gray-700">
                        <li>Optimize queries for efficiency</li>
                        <li>Use appropriate timeouts</li>
                        <li>Implement connection pooling</li>
                        <li>Report performance issues</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Scaling</h4>
                      <ul className="list-disc list-inside space-y-1 text-sm text-gray-700">
                        <li>Plan for growth and scale</li>
                        <li>Use enterprise solutions for large teams</li>
                        <li>Consider self-hosting for high volume</li>
                        <li>Contact us for custom arrangements</li>
                      </ul>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            {/* Service Availability */}
            <section>
              <h2 className="text-2xl font-bold mb-4 flex items-center gap-3">
                <AlertTriangle className="w-6 h-6" />
                Service Availability & Limitations
              </h2>
              
              <div className="space-y-6">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">Service Level Expectations</h3>
                  <div className="grid md:grid-cols-2 gap-6">
                    <div>
                      <h4 className="font-bold mb-3">Availability</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>We strive for <strong>99.9% uptime</strong> but cannot guarantee 100% availability</li>
                        <li><strong>Planned maintenance</strong> will be announced in advance</li>
                        <li><strong>Emergency maintenance</strong> may occur with minimal notice</li>
                        <li><strong>Self-hosted deployments</strong> are your responsibility to maintain</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Support</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li><strong>Community support</strong> through GitHub and documentation</li>
                        <li><strong>Email support</strong> for account and technical issues</li>
                        <li><strong>Enterprise support</strong> available for business customers</li>
                        <li><strong>Response times</strong> vary based on issue severity and plan</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="neo-border bg-gray-50 p-6">
                  <h3 className="text-xl font-bold mb-4">Important Disclaimers & Limitations</h3>
                  <div className="space-y-4">
                    <div className="neo-border bg-white p-4">
                      <h4 className="font-bold mb-3">AI Technology Limitations</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li><strong>AI-generated queries may contain errors</strong> or be suboptimal</li>
                        <li><strong>Always review and test queries</strong> before running on production data</li>
                        <li><strong>AI responses are based on training data</strong> and may not reflect current best practices</li>
                        <li><strong>Complex operations may require manual intervention</strong> and human expertise</li>
                        <li><strong>AI cannot replace proper database administration</strong> and security practices</li>
                      </ul>
                    </div>

                    <div className="neo-border bg-white p-4">
                      <h4 className="font-bold mb-3">Data Security & Risk</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li><strong>No system is 100% secure</strong> - implement multiple layers of protection</li>
                        <li><strong>You are responsible</strong> for securing your database credentials and access</li>
                        <li><strong>Regular backups</strong> are essential for data protection</li>
                        <li><strong>Test disaster recovery procedures</strong> regularly</li>
                        <li><strong>Monitor access logs</strong> and unusual activities</li>
                      </ul>
                    </div>

                    <div className="neo-border bg-white p-4">
                      <h4 className="font-bold mb-3">Service Dependencies</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li><strong>Third-party services</strong> (AI providers, cloud infrastructure) may affect availability</li>
                        <li><strong>Internet connectivity</strong> is required for cloud-hosted deployments</li>
                        <li><strong>Database connectivity</strong> issues are outside our control</li>
                        <li><strong>Browser compatibility</strong> may limit some features</li>
                      </ul>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            {/* Liability and Indemnification */}
            <section>
              <h2 className="text-2xl font-bold mb-4">Limitation of Liability & Indemnification</h2>
              <div className="space-y-6">
                <div className="neo-border bg-gray-50 p-6">
                  <h3 className="text-xl font-bold mb-4">Limitation of Liability</h3>
                  <div className="neo-border bg-[#FFDB58]/10 p-4 mb-4">
                    <p className="text-lg font-bold mb-2">IMPORTANT LEGAL NOTICE:</p>
                    <p className="text-gray-700">
                      To the fullest extent permitted by applicable law, Aurivance Technologies LLP shall not be liable for any damages arising from your use of NeoBase.
                    </p>
                  </div>
                  
                  <div className="grid md:grid-cols-2 gap-6">
                    <div>
                      <h4 className="font-semibold mb-2 text-gray-700">Excluded Damages</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li><strong>Indirect or consequential damages</strong> arising from service use</li>
                        <li><strong>Data loss or corruption</strong> due to user error or system failures</li>
                        <li><strong>Business interruption</strong> or lost profits from service downtime</li>
                        <li><strong>Damage from AI-generated queries</strong> or recommendations</li>
                        <li><strong>Security breaches</strong> not directly caused by our negligence</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-semibold mb-2 text-gray-700">Maximum Liability</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li><strong>Total liability limited</strong> to the amount you paid in the 12 months prior</li>
                        <li><strong>For free services,</strong> liability is limited to $100 USD</li>
                        <li><strong>No liability for self-hosted</strong> deployments beyond software defects</li>
                        <li><strong>Force majeure events</strong> exclude all liability</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">User Indemnification</h3>
                  <p className="text-lg leading-relaxed mb-4">
                    You agree to indemnify and hold harmless Aurivance Technologies LLP from claims arising from:
                  </p>
                  <div className="grid md:grid-cols-2 gap-6">
                    <ul className="list-disc list-inside space-y-2 text-gray-700">
                      <li><strong>Your use of the service</strong> in violation of these Terms</li>
                      <li><strong>Unauthorized database access</strong> or data breaches</li>
                      <li><strong>Violation of third-party rights</strong> including intellectual property</li>
                      <li><strong>Negligent or willful misconduct</strong> in using NeoBase</li>
                    </ul>
                    <ul className="list-disc list-inside space-y-2 text-gray-700">
                      <li><strong>Data protection violations</strong> or privacy law breaches</li>
                      <li><strong>Damage caused by your queries</strong> or database operations</li>
                      <li><strong>Misrepresentation</strong> in account registration or usage</li>
                      <li><strong>Failure to maintain security</strong> of your systems and credentials</li>
                    </ul>
                  </div>
                </div>
              </div>
            </section>

            {/* Account Management */}
            <section>
              <h2 className="text-2xl font-bold mb-4">Account Management & Termination</h2>
              <div className="grid md:grid-cols-2 gap-6">
                <div className="neo-border bg-gray-50 p-6">
                  <h3 className="text-xl font-bold mb-4">Your Rights</h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>Terminate account</strong> at any time without penalty</li>
                    <li><strong>Export your data</strong> before account closure</li>
                    <li><strong>Request data deletion</strong> after termination</li>
                    <li><strong>Self-hosted installations</strong> remain under your control</li>
                    <li><strong>Receive notice</strong> of any service changes affecting you</li>
                    <li><strong>Transfer data</strong> to other services or systems</li>
                  </ul>
                </div>
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">Our Rights</h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>Suspend or terminate accounts</strong> for Terms violations</li>
                    <li><strong>Modify or discontinue service</strong> with reasonable notice</li>
                    <li><strong>Immediate termination</strong> for security threats or illegal activity</li>
                    <li><strong>Retain data</strong> as outlined in our Privacy Policy</li>
                    <li><strong>Update Terms</strong> with notice and opportunity to object</li>
                    <li><strong>Refuse service</strong> for valid business or legal reasons</li>
                  </ul>
                </div>
              </div>

              <div className="mt-6 neo-border bg-[#FFDB58]/10 p-6">
                <h3 className="text-xl font-bold mb-4">Termination Process</h3>
                <div className="grid md:grid-cols-3 gap-6">
                  <div>
                    <h4 className="font-bold mb-3">Voluntary Termination</h4>
                    <ul className="list-disc list-inside space-y-1 text-sm text-gray-700">
                      <li>Account closure through settings</li>
                      <li>30-day data retention period</li>
                      <li>Export options before deletion</li>
                      <li>Confirmation required</li>
                    </ul>
                  </div>
                  <div>
                    <h4 className="font-bold mb-3">Involuntary Termination</h4>
                    <ul className="list-disc list-inside space-y-1 text-sm text-gray-700">
                      <li>Notice period when possible</li>
                      <li>Reason for termination provided</li>
                      <li>Appeal process available</li>
                      <li>Data export assistance</li>
                    </ul>
                  </div>
                  <div>
                    <h4 className="font-bold mb-3">Post-Termination</h4>
                    <ul className="list-disc list-inside space-y-1 text-sm text-gray-700">
                      <li>Surviving Terms remain in effect</li>
                      <li>Data deletion as per Privacy Policy</li>
                      <li>Self-hosted installations unaffected</li>
                      <li>Outstanding obligations remain</li>
                    </ul>
                  </div>
                </div>
              </div>
            </section>

            {/* Intellectual Property */}
            <section className="neo-border bg-gray-50 p-8">
              <h2 className="text-2xl font-bold mb-6">Intellectual Property Rights</h2>
              <div className="grid md:grid-cols-2 gap-6">
                <div>
                  <h3 className="text-xl font-bold mb-4">Our Intellectual Property</h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>NeoBase platform and software</strong> (subject to open-source license)</li>
                    <li><strong>Proprietary algorithms and AI models</strong> (not included in open source)</li>
                    <li><strong>Trademarks and brand elements</strong> including name and logos</li>
                    <li><strong>Documentation and training materials</strong> (CC license where applicable)</li>
                    <li><strong>Service architecture and infrastructure</strong> designs</li>
                  </ul>
                </div>
                <div>
                  <h3 className="text-xl font-bold mb-4">Open Source Components</h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>MIT License</strong> applies to the core NeoBase platform</li>
                    <li><strong>Third-party libraries</strong> retain their respective licenses</li>
                    <li><strong>Community contributions</strong> welcome under project license</li>
                    <li><strong>Commercial use permitted</strong> under open-source terms</li>
                    <li><strong>Attribution requirements</strong> as specified in license files</li>
                  </ul>
                </div>
              </div>
            </section>

            {/* Governing Law */}
            <section className="neo-border bg-white p-8">
              <h2 className="text-2xl font-bold mb-6">Governing Law & Dispute Resolution</h2>
              <div className="grid md:grid-cols-2 gap-6">
                <div>
                  <h3 className="text-xl font-bold mb-4">Jurisdiction</h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>Governed by Indian law</strong> where Aurivance Technologies LLP is incorporated</li>
                    <li><strong>Courts in Punjab, India</strong> have exclusive jurisdiction</li>
                    <li><strong>Disputes resolved</strong> under Indian legal procedures</li>
                    <li><strong>Self-hosted deployments</strong> may be subject to local laws</li>
                  </ul>
                </div>
                <div>
                  <h3 className="text-xl font-bold mb-4">Dispute Resolution</h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>Initial resolution</strong> through direct communication</li>
                    <li><strong>Mediation</strong> preferred for business disputes</li>
                    <li><strong>Arbitration</strong> available for complex matters</li>
                    <li><strong>Court proceedings</strong> as last resort</li>
                  </ul>
                </div>
              </div>
            </section>

            {/* Changes to Terms */}
            <section>
              <h2 className="text-2xl font-bold mb-4">Changes to These Terms</h2>
              <div className="neo-border bg-gray-50 p-6">
                <p className="text-lg leading-relaxed mb-4">
                  We may update these Terms from time to time to reflect changes in our service, business practices, or legal requirements:
                </p>
                <div className="grid md:grid-cols-2 gap-6">
                  <div>
                    <h3 className="text-xl font-bold mb-4">Notification Process</h3>
                    <ul className="list-disc list-inside space-y-2 text-gray-700">
                      <li><strong>Email notification</strong> to all registered users</li>
                      <li><strong>In-app notification</strong> on next login</li>
                      <li><strong>Website posting</strong> with prominent notice</li>
                      <li><strong>30-day advance notice</strong> for material changes</li>
                      <li><strong>Immediate effect</strong> only for legal compliance or security</li>
                    </ul>
                  </div>
                  <div>
                    <h3 className="text-xl font-bold mb-4">Your Options</h3>
                    <ul className="list-disc list-inside space-y-2 text-gray-700">
                      <li><strong>Review changes</strong> before they take effect</li>
                      <li><strong>Contact us</strong> with questions or concerns</li>
                      <li><strong>Terminate account</strong> if you disagree with changes</li>
                      <li><strong>Export data</strong> before termination</li>
                      <li><strong>Continued use</strong> constitutes acceptance of new Terms</li>
                    </ul>
                  </div>
                </div>
              </div>
            </section>

            {/* Contact Information */}
            <section className="neo-border bg-[#FFDB58]/10 p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <Mail className="w-8 h-8" />
                Contact Us
              </h2>
              
              <p className="text-lg leading-relaxed mb-6">
                If you have questions, concerns, or comments about these Terms of Service, please contact us:
              </p>
              
              <div className="grid md:grid-cols-2 gap-8">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4 flex items-center gap-2">
                    <Building className="w-6 h-6" />
                    Company Information
                  </h3>
                  <div className="space-y-4">
                    <div className="flex items-start gap-3">
                      <Building className="w-5 h-5 mt-0.5 flex-shrink-0" />
                      <div>
                        <p className="font-bold">Aurivance Technologies LLP</p>
                        <p className="text-gray-600">Limited Liability Partnership</p>
                      </div>
                    </div>
                    <div className="flex items-start gap-3">
                      <MapPin className="w-5 h-5 mt-0.5 flex-shrink-0" />
                      <div>
                        <p className="font-bold">Registered Address:</p>
                        <p className="text-gray-600">1035 1st floor, Global City Sector 124<br />Kharar(Rupnagar), Punjab, India, 140301</p>
                      </div>
                    </div>
                  </div>
                </div>
                
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4 flex items-center gap-2">
                    <Mail className="w-6 h-6" />
                    Contact Details
                  </h3>
                  <div className="space-y-4">
                    <div className="flex items-center gap-3">
                      <Mail className="w-5 h-5 flex-shrink-0" />
                      <div>
                        <p className="font-bold">Email:</p>
                        <div className="space-y-1">
                          <p><a href="mailto:hi@neobase.cloud" className="hover:underline font-medium">hi@neobase.cloud</a></p>
                          <p><a href="mailto:office@aurivancetech.com" className="hover:underline font-medium">office@aurivancetech.com</a></p>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <Phone className="w-5 h-5 flex-shrink-0" />
                      <div>
                        <p className="font-bold">Phone:</p>
                        <div className="space-y-1">
                          <p><a href="tel:+917004297500" className="hover:underline font-medium">+91 7004297500</a></p>
                          <p><a href="tel:+919877253751" className="hover:underline font-medium">+91 9877253751</a></p>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <Globe className="w-5 h-5 flex-shrink-0" />
                      <div>
                        <p className="font-bold">Website:</p>
                        <p><a href="https://neobase.cloud" className="hover:underline font-medium">https://neobase.cloud</a></p>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            {/* Effective Date */}
            <section className="neo-border bg-gray-50 p-8">
              <h2 className="text-2xl font-bold mb-6">Effective Date & Acknowledgment</h2>
              <div className="grid md:grid-cols-2 gap-6">
                <div>
                  <p className="text-lg"><strong>Effective Date:</strong> August 24, 2025</p>
                  <p className="text-lg"><strong>Version:</strong> 1.0</p>
                  <p className="text-lg"><strong>Last Review:</strong> August 24, 2025</p>
                </div>
                <div>
                  <p className="text-gray-600">
                    By using NeoBase after the effective date, you acknowledge that you have read, understood, and agree to be bound by these Terms of Service. 
                    These Terms supersede all previous agreements and understandings between you and Aurivance Technologies LLP regarding NeoBase.
                  </p>
                </div>
              </div>
            </section>
          </div>
        </div>
      </main>
      
      <Footer />
    </div>
  );
};

export default TermsOfService;