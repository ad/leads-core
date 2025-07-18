/**
 * UI utilities and components for Leads Core Admin Panel
 */

class UIManager {
    constructor() {
        this.toastContainer = null;
        this.loadingElement = null;
        this.init();
    }

    /**
     * Initialize UI manager
     */
    init() {
        this.loadingElement = document.getElementById('loading');
        this.toastContainer = document.getElementById('toast-container');
        this.setupModalEvents();
    }

    /**
     * Setup global modal events
     */
    setupModalEvents() {
        // Close modal when clicking outside
        document.addEventListener('click', (e) => {
            if (e.target.classList.contains('modal')) {
                this.closeAllModals();
            }
        });

        // Close modal with Escape key
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                this.closeAllModals();
            }
        });
    }

    /**
     * Show loading screen
     */
    showLoading() {
        if (this.loadingElement) {
            this.loadingElement.style.display = 'flex';
        }
    }

    /**
     * Hide loading screen
     */
    hideLoading() {
        if (this.loadingElement) {
            this.loadingElement.style.display = 'none';
        }
    }

    /**
     * Show toast notification
     */
    showToast(message, type = 'info', duration = 5000) {
        if (!this.toastContainer) return;

        let title = '';
        switch (type) {
            case 'success':
                title = 'Success';
                break;
            case 'error':
                title = 'Error';
                break;
            case 'warning':
                title = 'Warning';
                break;
            default:
                title = 'Info';
        }
        
        const toast = document.createElement('div');
        toast.className = `toast ${type}`;
        toast.innerHTML = `
            <label for="t-success" class="close" onclick="this.parentNode.remove()"></label>
			<h3>${title}</h3>
			<p>${this.escapeHtml(message)}!</p>
        `;
        
        this.toastContainer.appendChild(toast);
        
        // Auto remove after duration
        setTimeout(() => {
            if (toast.parentNode) {
                toast.parentNode.removeChild(toast);
            }
        }, duration);
    }

    /**
     * Show error message
     */
    showError(message) {
        const errorElement = document.getElementById('login-error');
        if (errorElement) {
            errorElement.textContent = message;
            errorElement.style.display = 'block';
        }
    }

    /**
     * Hide error message
     */
    hideError() {
        const errorElement = document.getElementById('login-error');
        if (errorElement) {
            errorElement.style.display = 'none';
        }
    }

    /**
     * Close all open modals
     */
    closeAllModals() {
        const modals = document.querySelectorAll('.modal');
        modals.forEach(modal => {
            modal.style.display = 'none';
        });
        
        // Reset widget managers state
        if (window.WidgetsManager) {
            window.WidgetsManager.currentWidgetId = null;
            window.WidgetsManager.widgetToDelete = null;
        }
    }

    /**
     * Format number with locale formatting
     */
    formatNumber(num) {
        if (typeof num !== 'number') return num;
        return new Intl.NumberFormat().format(num);
    }

    /**
     * Format date with locale formatting
     */
    formatDate(dateString) {
        if (!dateString) return 'N/A';
        try {
            return new Date(dateString).toLocaleDateString('en-US', {
                year: 'numeric',
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        } catch {
            return dateString;
        }
    }

    /**
     * Escape HTML to prevent XSS
     */
    escapeHtml(text) {
        if (typeof text !== 'string') return text;
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * Show widget details modal
     */
    async showWidgetDetails(widgetId) {
        try {
            this.showLoading();
            
            const widget = await window.APIClient.getWidget(widgetId);
            const widgetStats = await window.APIClient.getWidgetStats(widgetId);
            
            // Try to get recent submissions
            let recentSubmissions = null;
            try {
                const submissionsResponse = await window.APIClient.getRecentSubmissions(widgetId, 3);
                recentSubmissions = submissionsResponse.data || [];
            } catch (error) {
                console.log('Could not load recent submissions:', error.message);
            }
            
            const modal = document.getElementById('widget-modal');
            const title = document.getElementById('modal-title');
            const contentData = document.getElementById('modal-content-data');
            const contentLoading = document.getElementById('modal-content-loading');
            
            if (modal && title && contentData && contentLoading) {
                title.textContent = `${widget.name || 'Untitled Widget'} - Details`;
                
                contentData.innerHTML = `
                    <div class="widget-details">
                        <div class="detail-section">
                            <h4>📋 Basic Information</h4>
                            <div class="detail-grid">
                                <div class="detail-item">
                                    <label>ID:</label>
                                    <span>${widget.id}</span>
                                </div>
                                <div class="detail-item">
                                    <label>Name:</label>
                                    <span>${widget.name || 'N/A'}</span>
                                </div>
                                <div class="detail-item">
                                    <label>Type:</label>
                                    <span>${widget.type || 'N/A'}</span>
                                </div>
                                <div class="detail-item">
                                    <label>Status:</label>
                                    <span class="status ${widget.isVisible ? 'enabled' : 'disabled'}">
                                        ${widget.isVisible ? '✅ Active' : '❌ Disabled'}
                                    </span>
                                </div>
                                <div class="detail-item">
                                    <label>Created:</label>
                                    <span>${this.formatDate(widget.created_at)}</span>
                                </div>
                                <div class="detail-item">
                                    <label>Updated:</label>
                                    <span>${this.formatDate(widget.updated_at)}</span>
                                </div>
                            </div>
                        </div>
                        
                        <div class="detail-section">
                            <h4>📊 Statistics</h4>
                            <div class="stats-grid">
                                <div class="stat-card">
                                    <div class="stat-value">${this.formatNumber(widgetStats.views || 0)}</div>
                                    <div class="stat-label">Total Views</div>
                                </div>
                                <div class="stat-card">
                                    <div class="stat-value">${this.formatNumber(widgetStats.submits || 0)}</div>
                                    <div class="stat-label">Submits</div>
                                </div>
                                <div class="stat-card">
                                    <div class="stat-value">${widgetStats.conversion_rate || '0'}%</div>
                                    <div class="stat-label">Conversion Rate</div>
                                </div>
                            </div>
                        </div>
                        
                        <div class="detail-section">
                            <h4>🧪 Widget Testing</h4>
                            <div class="widget-testing-container">
                                <div class="testing-section">
                                    <h5>📡 Send Events</h5>
                                    <div class="event-buttons">
                                        <button type="button" class="btn btn-sm btn-secondary" onclick="window.UI.sendWidgetEvent('${widget.id}', 'view', this)">
                                            👀 Send View Event
                                        </button>
                                        <button type="button" class="btn btn-sm btn-secondary" onclick="window.UI.sendWidgetEvent('${widget.id}', 'close', this)">
                                            ❌ Send Close Event
                                        </button>
                                    </div>
                                </div>
                                
                                ${widget.config && Object.keys(widget.config).length > 0 ? `
                                <div class="testing-section">
                                    <h5>📝 Test Widget Submission</h5>
                                    <form id="test-widget-${widget.id}" class="test-widget">
                                        ${Object.entries(widget.config).map(([key, field]) => this.generateTestField({name: key, ...field})).join('')}
                                        <div class="test-widget-actions">
                                            <button type="submit" class="btn btn-sm btn-primary">
                                                🚀 Submit Test Data
                                            </button>
                                            <button type="button" class="btn btn-sm btn-outline" onclick="window.UI.fillRandomTestData('${widget.id}')">
                                                🎲 Fill Random Data
                                            </button>
                                        </div>
                                    </form>
                                </div>
                                ` : `
                                <div class="testing-section">
                                    <h5>📝 Test Widget Submission</h5>
                                    <p class="no-fields-message">This widget has no fields configured.</p>
                                    <form id="test-widget-${widget.id}" class="test-widget">
                                        <div class="widget-group">
                                            <label for="test-field-name">Test Name</label>
                                            <input type="text" id="test-field-name" name="name" placeholder="Enter test name">
                                        </div>
                                        <div class="widget-group">
                                            <label for="test-field-email">Test Email</label>
                                            <input type="email" id="test-field-email" name="email" placeholder="Enter test email">
                                        </div>
                                        <div class="test-widget-actions">
                                            <button type="submit" class="btn btn-sm btn-primary">
                                                🚀 Submit Test Data
                                            </button>
                                            <button type="button" class="btn btn-sm btn-outline" onclick="window.UI.fillRandomTestData('${widget.id}')">
                                                🎲 Fill Random Data
                                            </button>
                                        </div>
                                    </form>
                                </div>
                                `}
                            </div>
                        </div>
                        
                        ${recentSubmissions && recentSubmissions.length > 0 ? `
                        <div class="detail-section">
                            <h4>📬 Recent Submits</h4>
                            <div class="submissions-list">
                                ${recentSubmissions.map(submission => `
                                    <div class="submission-item">
                                        <div class="submission-header">
                                            <span class="submission-date">${this.formatDate(submission.created_at)}</span>
                                            <span class="submission-id">#${submission.id}</span>
                                        </div>
                                        <div class="submission-data">
                                            ${Object.entries(submission.data || {}).map(([key, value]) => `
                                                <div class="data-item">
                                                    <span class="data-key">${key}:</span>
                                                    <span class="data-value">${this.escapeHtml(String(value))}</span>
                                                </div>
                                            `).join('')}
                                        </div>
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                        ` : ''}
                        
                        ${widget.config && widget.config.length > 0 ? `
                        <div class="detail-section">
                            <h4>📝 Widget Config</h4>
                            <div class="fields-list">
                                ${widget.config.map(field => `
                                    <div class="field-item">
                                        <div class="field-header">
                                            <span class="field-name">${field.name}</span>
                                            <span class="field-type">${field.type}</span>
                                            ${field.required ? '<span class="field-required">Required</span>' : ''}
                                        </div>
                                        ${field.description ? `<div class="field-description">${field.description}</div>` : ''}
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                        ` : ''}
                    </div>
                `;
                
                contentLoading.style.display = 'none';
                contentData.style.display = 'block';
                modal.style.display = 'flex';
                
                // Setup close button
                const closeBtn = document.getElementById('modal-close');
                if (closeBtn) {
                    closeBtn.onclick = () => {
                        modal.style.display = 'none';
                        contentData.style.display = 'none';
                        contentLoading.style.display = 'block';
                    };
                }

                // Setup test widget submission
                const testWidget = document.getElementById(`test-widget-${widgetId}`);
                if (testWidget) {
                    testWidget.onsubmit = (e) => {
                        e.preventDefault();
                        this.submitTestWidget(widgetId);
                    };
                }
            }
            
        } catch (error) {
            console.error('Error loading widget details:', error);
            this.showToast('Failed to load widget details', 'error');
        } finally {
            this.hideLoading();
        }
    }

    /**
     * Generate test field HTML based on field configuration
     */
    generateTestField(field) {
        const fieldId = `test-field-${field.name}`;
        const required = field.required ? 'required' : '';
        const placeholder = field.placeholder || `Enter ${field.name}`;

        switch (field.type) {
            case 'email':
                return `
                    <div class="widget-group">
                        <label for="${fieldId}">${field.name}${field.required ? ' *' : ''}</label>
                        <input type="email" id="${fieldId}" name="${field.name}" placeholder="${placeholder}" ${required}>
                    </div>
                `;
            case 'tel':
                return `
                    <div class="widget-group">
                        <label for="${fieldId}">${field.name}${field.required ? ' *' : ''}</label>
                        <input type="tel" id="${fieldId}" name="${field.name}" placeholder="${placeholder}" ${required}>
                    </div>
                `;
            case 'textarea':
                return `
                    <div class="widget-group">
                        <label for="${fieldId}">${field.name}${field.required ? ' *' : ''}</label>
                        <textarea id="${fieldId}" name="${field.name}" placeholder="${placeholder}" rows="3" ${required}></textarea>
                    </div>
                `;
            case 'select':
                const options = field.options || [];
                return `
                    <div class="widget-group">
                        <label for="${fieldId}">${field.name}${field.required ? ' *' : ''}</label>
                        <select id="${fieldId}" name="${field.name}" ${required}>
                            <option value="">Choose option</option>
                            ${options.map(option => `<option value="${option}">${option}</option>`).join('')}
                        </select>
                    </div>
                `;
            case 'checkbox':
                return `
                    <div class="widget-group">
                        <div class="checkbox-group">
                            <input type="checkbox" id="${fieldId}" name="${field.name}" value="1">
                            <label for="${fieldId}">${field.name}</label>
                        </div>
                    </div>
                `;
            case 'radio':
                const radioOptions = field.options || [];
                return `
                    <div class="widget-group">
                        <label>${field.name}${field.required ? ' *' : ''}</label>
                        <div class="radio-group">
                            ${radioOptions.map((option, index) => `
                                <div class="radio-item">
                                    <input type="radio" id="${fieldId}-${index}" name="${field.name}" value="${option}" ${required}>
                                    <label for="${fieldId}-${index}">${option}</label>
                                </div>
                            `).join('')}
                        </div>
                    </div>
                `;
            default:
                return `
                    <div class="widget-group">
                        <label for="${fieldId}">${field.name}${field.required ? ' *' : ''}</label>
                        <input type="text" id="${fieldId}" name="${field.name}" placeholder="${placeholder}" ${required}>
                    </div>
                `;
        }
    }

    /**
     * Send widget event (view or close)
     */
    async sendWidgetEvent(widgetId, eventType, buttonElement = null) {
        try {
            if (buttonElement) {
                this.setButtonLoading(buttonElement, true);
            }
            await window.APIClient.sendWidgetEvent(widgetId, eventType);
            this.showToast(`${eventType.charAt(0).toUpperCase() + eventType.slice(1)} event sent successfully!`, 'success');
            
            // Refresh statistics in the background
            setTimeout(() => {
                const currentModal = document.getElementById('widget-modal');
                if (currentModal && currentModal.style.display === 'flex') {
                    this.refreshWidgetStats(widgetId);
                }
            }, 500);
        } catch (error) {
            console.error('Error sending widget event:', error);
            this.showToast(`Failed to send ${eventType} event: ${error.message}`, 'error');
        } finally {
            if (buttonElement) {
                this.setButtonLoading(buttonElement, false);
            }
        }
    }

    /**
     * Submit test widget data
     */
    async submitTestWidget(widgetId) {
        try {
            const widget = document.getElementById(`test-widget-${widgetId}`);
            if (!widget) return;

            const widgetData = new FormData(widget);
            const testData = {};
            
            // Convert WidgetData to plain object
            for (let [key, value] of widgetData.entries()) {
                testData[key] = value;
            }

            // Wrap data in the expected format
            const submissionData = {
                data: testData
            };

            const submitBtn = widget.querySelector('button[type="submit"]');
            this.setButtonLoading(submitBtn, true);

            await window.APIClient.submitTestWidgetData(widgetId, submissionData);
            this.showToast('Test widget data submitted successfully!', 'success');
            
            // Reset widget after successful submission
            widget.reset();
            
            // Refresh the modal with updated statistics
            setTimeout(() => {
                this.showWidgetDetails(widgetId);
            }, 1000);
            
        } catch (error) {
            console.error('Error submitting test widget:', error);
            this.showToast(`Failed to submit test widget: ${error.message}`, 'error');
        } finally {
            const widget = document.getElementById(`test-widget-${widgetId}`);
            const submitBtn = widget?.querySelector('button[type="submit"]');
            this.setButtonLoading(submitBtn, false);
        }
    }

    /**
     * Fill widget with random test data
     */
    fillRandomTestData(widgetId) {
        const widget = document.getElementById(`test-widget-${widgetId}`);
        if (!widget) return;

        const inputs = widget.querySelectorAll('input, textarea, select');
        
        inputs.forEach(input => {
            if (input.type === 'checkbox') {
                input.checked = Math.random() > 0.5;
            } else if (input.type === 'radio') {
                // For radio buttons, randomly select one option per group
                const radioGroup = widget.querySelectorAll(`input[name="${input.name}"]`);
                const randomIndex = Math.floor(Math.random() * radioGroup.length);
                radioGroup.forEach((radio, index) => {
                    radio.checked = index === randomIndex;
                });
            } else if (input.tagName === 'SELECT') {
                const options = input.querySelectorAll('option');
                if (options.length > 1) {
                    const randomIndex = Math.floor(Math.random() * (options.length - 1)) + 1;
                    input.selectedIndex = randomIndex;
                }
            } else {
                // Generate random data based on field type and name
                input.value = this.generateRandomValue(input);
            }
        });

        this.showToast('Random test data filled!', 'info');
    }

    /**
     * Generate random value based on input type and name
     */
    generateRandomValue(input) {
        const fieldName = input.name.toLowerCase();
        
        if (input.type === 'email' || fieldName.includes('email')) {
            const domains = ['example.com', 'test.com', 'demo.org'];
            const randomDomain = domains[Math.floor(Math.random() * domains.length)];
            return `test${Math.floor(Math.random() * 1000)}@${randomDomain}`;
        }
        
        if (input.type === 'tel' || fieldName.includes('phone') || fieldName.includes('tel')) {
            return `+1${Math.floor(Math.random() * 9000000000) + 1000000000}`;
        }
        
        if (fieldName.includes('name')) {
            const names = ['John Doe', 'Jane Smith', 'Bob Johnson', 'Alice Brown', 'Charlie Wilson'];
            return names[Math.floor(Math.random() * names.length)];
        }
        
        if (fieldName.includes('company') || fieldName.includes('organization')) {
            const companies = ['Acme Corp', 'TechCorp Inc', 'Global Solutions', 'Innovation Labs', 'Future Systems'];
            return companies[Math.floor(Math.random() * companies.length)];
        }
        
        if (input.tagName === 'TEXTAREA' || fieldName.includes('message') || fieldName.includes('comment')) {
            const messages = [
                'This is a test message for widget validation.',
                'Great product! I would like to know more information.',
                'Please contact me regarding your services.',
                'I have some questions about your offerings.',
                'Looking forward to hearing from you soon.'
            ];
            return messages[Math.floor(Math.random() * messages.length)];
        }
        
        // Default random text
        const randomWords = ['test', 'sample', 'demo', 'example', 'placeholder'];
        const randomWord = randomWords[Math.floor(Math.random() * randomWords.length)];
        return `${randomWord} ${Math.floor(Math.random() * 1000)}`;
    }

    /**
     * Refresh widget statistics without reopening the modal
     */
    async refreshWidgetStats(widgetId) {
        try {
            const widgetStats = await window.APIClient.getWidgetStats(widgetId);
            
            // Update the statistics in the current modal
            const viewsElement = document.querySelector('.stats-grid .stat-card:nth-child(1) .stat-value');
            const submissionsElement = document.querySelector('.stats-grid .stat-card:nth-child(2) .stat-value');
            const conversionElement = document.querySelector('.stats-grid .stat-card:nth-child(3) .stat-value');
            
            if (viewsElement) viewsElement.textContent = this.formatNumber(widgetStats.views || 0);
            if (submissionsElement) submissionsElement.textContent = this.formatNumber(widgetStats.submits || 0);
            if (conversionElement) conversionElement.textContent = `${widgetStats.conversion_rate || '0'}%`;
            
        } catch (error) {
            console.error('Error refreshing widget stats:', error);
        }
    }

    /**
     * Update page title
     */
    updatePageTitle(title) {
        document.title = `${title} - Leads Core Admin Panel`;
    }

    /**
     * Set button loading state
     */
    setButtonLoading(button, loading) {
        if (!button) return;
        
        const btnText = button.querySelector('.btn-text');
        const btnLoading = button.querySelector('.btn-loading');
        
        if (loading) {
            button.disabled = true;
            if (btnText) btnText.style.display = 'none';
            if (btnLoading) btnLoading.style.display = 'inline';
        } else {
            button.disabled = false;
            if (btnText) btnText.style.display = 'inline';
            if (btnLoading) btnLoading.style.display = 'none';
        }
    }

    /**
     * Debounce function to limit function calls
     */
    debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    }
}

// Create global instance
window.UI = new UIManager();
