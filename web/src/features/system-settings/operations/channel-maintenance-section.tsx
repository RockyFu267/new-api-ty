import { ChatPromptManagementSection } from './chat-prompt-management-section'
import { KnowledgeDocumentManagementSection } from './knowledge-document-management-section'

export function ChannelMaintenanceSection() {
  return (
    <div className='space-y-8'>
      <ChatPromptManagementSection />
      <KnowledgeDocumentManagementSection />
    </div>
  )
}
