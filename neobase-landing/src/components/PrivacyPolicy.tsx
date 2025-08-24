import { Link } from 'react-router-dom';
import { ArrowLeft, Shield, Database, Lock, Eye, UserCheck, Globe, Mail, Building, Phone, MapPin } from 'lucide-react';
import Navbar from './Navbar';
import Footer from './Footer';
import FloatingBackground from './FloatingBackground';

const PrivacyPolicy = () => {
  return (
    <div className="min-h-screen bg-[#FFDB58]/10">
      <Navbar forks={0} />
      
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
                <Shield className="w-8 h-8" />
              </div>
            </div>
            <h1 className="text-4xl md:text-4xl font-bold mb-4">Privacy Policy</h1>
            <p className="text-base text-gray-600">Last updated: August 24, 2025</p>
          </div>

          <div className="space-y-12">
            {/* Introduction */}
            <section className="neo-border bg-[#FFDB58]/10 p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <Eye className="w-8 h-8" />
                Our Commitment to Your Privacy
              </h2>
              <p className="text-lg leading-relaxed mb-4">
                At NeoBase, your privacy is our top priority. This Privacy Policy explains how Aurivance Technologies LLP ("we," "us," or "our") collects, uses, protects, and handles your information when you use NeoBase, our AI Data Copilot platform. We are committed to transparency and ensuring your data remains secure and under your complete control.
              </p>
              <div className="neo-border bg-white p-4">
                <p className="text-base leading-relaxed font-medium">
                  <strong>Key Principle:</strong> We believe your data belongs to you. We do not sell, rent, or use your database content for training our AI models or for any purpose other than providing you with our service.
                </p>
              </div>
            </section>

            {/* Company Information */}
            <section className="neo-border bg-gray-50 p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <Building className="w-8 h-8" />
                About Aurivance Technologies LLP
              </h2>
              <p className="text-lg leading-relaxed mb-6">
                NeoBase is developed and operated by Aurivance Technologies LLP, a technology company incorporated in India, specializing in AI-powered data solutions and enterprise software development.
              </p>
              <div className="grid md:grid-cols-2 gap-6">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">Company Details</h3>
                  <ul className="space-y-2 text-gray-700">
                    <li><strong>Legal Name:</strong> Aurivance Technologies LLP</li>
                    <li><strong>Registration:</strong> Limited Liability Partnership</li>
                    <li><strong>Jurisdiction:</strong> India</li>
                    <li><strong>Primary Business:</strong> AI & Data Technology Solutions</li>
                  </ul>
                </div>
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">Service Focus</h3>
                  <ul className="space-y-2 text-gray-700">
                    <li>AI-powered database assistance</li>
                    <li>Enterprise data management</li>
                    <li>Self-hosted solutions</li>
                    <li>Open-source technology</li>
                  </ul>
                </div>
              </div>
            </section>

            {/* Data Collection */}
            <section className="neo-border bg-gray-50 p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <Database className="w-8 h-8" />
                Information We Collect
              </h2>
              
              <div className="space-y-6">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4 flex items-center gap-2">
                    <UserCheck className="w-6 h-6" />
                    1. Account & Authentication Data
                  </h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>Basic Account Info:</strong> Username, encrypted password, account creation timestamp</li>
                    <li><strong>Authentication Tokens:</strong> Secure session tokens for maintaining login state</li>
                    <li><strong>User Preferences:</strong> Application settings, theme preferences, language choices</li>
                    <li><strong>Activity Timestamps:</strong> Last login, last active session, account updates</li>
                  </ul>
                </div>

                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4 flex items-center gap-2">
                    <Database className="w-6 h-6" />
                    2. Database Connection Information
                  </h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>Connection Parameters:</strong> Database host, port, database names, connection types</li>
                    <li><strong>Credentials:</strong> Usernames and passwords (encrypted using AES-256)</li>
                    <li><strong>SSL Configuration:</strong> SSL certificates, connection security settings</li>
                    <li><strong>Schema Information:</strong> Table structures, column definitions, relationships (for AI query generation)</li>
                    <li><strong>Connection Metadata:</strong> Connection nicknames, descriptions, creation dates</li>
                  </ul>
                </div>

                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4 flex items-center gap-2">
                    <Mail className="w-6 h-6" />
                    3. Chat & Interaction Data
                  </h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>User Queries:</strong> Your natural language questions and requests</li>
                    <li><strong>AI Responses:</strong> Generated explanations, suggestions, and recommendations</li>
                    <li><strong>Generated Queries:</strong> SQL and database queries created by our AI</li>
                    <li><strong>Query Results:</strong> Metadata about query execution (not actual data content)</li>
                    <li><strong>Chat History:</strong> Conversation threads, timestamps, message context</li>
                    <li><strong>User Feedback:</strong> Ratings, corrections, and improvement suggestions</li>
                  </ul>
                </div>
              </div>
            </section>

            {/* How We Use Data */}
            <section className="neo-border bg-white p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <UserCheck className="w-8 h-8" />
                How We Use Your Information
              </h2>
              
              <div className="grid md:grid-cols-2 gap-6">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4 flex items-center gap-2">
                    <Database className="w-6 h-6" />
                    Core Service Operations
                  </h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>AI Query Generation:</strong> Process your natural language requests to generate appropriate database queries</li>
                    <li><strong>Database Connectivity:</strong> Establish and maintain secure connections to your databases</li>
                    <li><strong>Data Analysis:</strong> Provide insights, suggestions, and explanations about your data</li>
                    <li><strong>Context Preservation:</strong> Maintain conversation history for better AI responses</li>
                    <li><strong>Authentication:</strong> Secure access control and user session management</li>
                  </ul>
                </div>
                
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4 flex items-center gap-2">
                    <Lock className="w-6 h-6" />
                    Security & Reliability
                  </h3>
                  <ul className="list-disc list-inside space-y-2 text-gray-700">
                    <li><strong>Threat Detection:</strong> Monitor for suspicious activities and potential security breaches</li>
                    <li><strong>System Monitoring:</strong> Ensure service reliability and optimal performance</li>
                    <li><strong>Error Resolution:</strong> Debug issues and improve system stability</li>
                    <li><strong>Backup & Recovery:</strong> Maintain data integrity and service continuity</li>
                    <li><strong>Compliance:</strong> Meet security standards and regulatory requirements</li>
                  </ul>
                </div>
              </div>
            </section>

            {/* Data Protection */}
            <section className="neo-border bg-gray-50 p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <Lock className="w-8 h-8" />
                How We Protect Your Data
              </h2>
              
              <div className="grid gap-6">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">🔐 Advanced Encryption</h3>
                  <div className="grid md:grid-cols-2 gap-6">
                    <div>
                      <h4 className="font-bold mb-3">Data at Rest</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>AES-256-GCM encryption for all stored data</li>
                        <li>Separate encryption keys for each data type</li>
                        <li>Database credentials encrypted with unique keys</li>
                        <li>Spreadsheet data encrypted before storage</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Data in Transit</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>TLS 1.3 for all client-server communication</li>
                        <li>Encrypted database connections (SSL/TLS)</li>
                        <li>Secure OAuth flows for third-party integrations</li>
                        <li>Certificate pinning for additional security</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="neo-border bg-[#FFDB58]/10 p-6">
                  <h3 className="text-xl font-bold mb-4">🏢 Self-Hosting for Maximum Security</h3>
                  <p className="text-lg leading-relaxed mb-4">
                    For organizations requiring complete data control, NeoBase offers self-hosted deployment options:
                  </p>
                  <div className="grid md:grid-cols-2 gap-6">
                    <div>
                      <h4 className="font-bold mb-3">Complete Control</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>Data never leaves your infrastructure</li>
                        <li>Your own encryption keys and certificates</li>
                        <li>Custom security policies and configurations</li>
                        <li>Full audit logs and monitoring</li>
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-bold mb-3">Deployment Options</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li>Docker containers for easy deployment</li>
                        <li>Kubernetes orchestration support</li>
                        <li>Cloud provider agnostic</li>
                        <li>On-premises installation guides</li>
                      </ul>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            {/* Data Sharing */}
            <section className="neo-border bg-white p-8">
              <h2 className="text-2xl font-bold mb-6 flex items-center gap-3">
                <Globe className="w-8 h-8" />
                Data Sharing and Third Parties
              </h2>
              
              <div className="space-y-6">
                <div className="neo-border bg-white p-6">
                  <h3 className="text-xl font-bold mb-4">❌ What We DO NOT Do</h3>
                  <div className="grid md:grid-cols-2 gap-6">
                    <ul className="list-disc list-inside space-y-2 text-gray-700">
                      <li><strong>Never sell or rent your data</strong> to third parties</li>
                      <li><strong>Never use your database content</strong> to train our AI models</li>
                      <li><strong>Never share sensitive data</strong> with unauthorized parties</li>
                      <li><strong>Never access your databases</strong> without explicit permission</li>
                    </ul>
                    <ul className="list-disc list-inside space-y-2 text-gray-700">
                      <li><strong>Never use personal data for marketing</strong> without consent</li>
                      <li><strong>Never share data with competitors</strong> or similar services</li>
                      <li><strong>Never use data for purposes</strong> other than service delivery</li>
                      <li><strong>Never retain data longer</strong> than necessary</li>
                    </ul>
                  </div>
                </div>

                <div className="neo-border bg-gray-50 p-6">
                  <h3 className="text-xl font-bold mb-4">✅ Limited, Necessary Sharing</h3>
                  <div className="space-y-4">
                    <div className="neo-border bg-white p-4">
                      <h4 className="font-bold mb-3">AI Service Providers (OpenAI, Google)</h4>
                      <ul className="list-disc list-inside space-y-2 text-gray-700">
                        <li><strong>What we share:</strong> Database schema information (table/column names, types) and your natural language queries</li>
                        <li><strong>What we DON'T share:</strong> Actual data content, connection credentials, or sensitive information</li>
                        <li><strong>Purpose:</strong> Generate appropriate database queries based on your requests</li>
                        <li><strong>Your control:</strong> You can opt out of AI features in settings</li>
                      </ul>
                    </div>
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
                If you have questions, concerns, or requests about this Privacy Policy or how we handle your data, please don't hesitate to contact us:
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
              <h2 className="text-2xl font-bold mb-6">Effective Date & Version</h2>
              <div className="grid md:grid-cols-2 gap-6">
                <div>
                  <p className="text-base"><strong>Effective Date:</strong> August 24, 2025</p>
                  <p className="text-baseg"><strong>Version:</strong> 1.0</p>
                  <p className="text-base"><strong>Last Review:</strong> August 24, 2025</p>
                </div>
                <div>
                  <p className="text-gray-600">
                    This Privacy Policy supersedes all previous versions. By continuing to use NeoBase after the effective date, you acknowledge that you have read, understood, and agree to be bound by this Privacy Policy.
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

export default PrivacyPolicy;