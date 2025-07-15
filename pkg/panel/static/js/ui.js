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
        
        const toast = document.createElement('div');
        toast.className = `toast ${type}`;
        toast.innerHTML = `
            <div class="toast-title">${this.escapeHtml(message)}</div>
            <button class="toast-close" onclick="this.parentNode.remove()">×</button>
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
        
        // Reset form managers state
        if (window.FormsManager) {
            window.FormsManager.currentFormId = null;
            window.FormsManager.formToDelete = null;
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
     * Show form details modal
     */
    async showFormDetails(formId) {
        try {
            this.showLoading();
            
            const form = await window.APIClient.getForm(formId);
            const formStats = await window.APIClient.getFormStats(formId);
            
            const modal = document.getElementById('form-modal');
            const title = document.getElementById('modal-title');
            const contentData = document.getElementById('modal-content-data');
            const contentLoading = document.getElementById('modal-content-loading');
            
            if (modal && title && contentData && contentLoading) {
                title.textContent = `${form.name || 'Untitled Form'} - Details`;
                
                contentData.innerHTML = `
                    <div class="form-details">
                        <div class="detail-section">
                            <h4>📋 Basic Information</h4>
                            <div class="detail-grid">
                                <div class="detail-item">
                                    <label>ID:</label>
                                    <span>${form.id}</span>
                                </div>
                                <div class="detail-item">
                                    <label>Name:</label>
                                    <span>${form.name || 'N/A'}</span>
                                </div>
                                <div class="detail-item">
                                    <label>Type:</label>
                                    <span>${form.type || 'N/A'}</span>
                                </div>
                                <div class="detail-item">
                                    <label>Status:</label>
                                    <span class="status ${form.enabled ? 'enabled' : 'disabled'}">
                                        ${form.enabled ? '✅ Active' : '❌ Disabled'}
                                    </span>
                                </div>
                                <div class="detail-item">
                                    <label>Created:</label>
                                    <span>${this.formatDate(form.created_at)}</span>
                                </div>
                                <div class="detail-item">
                                    <label>Updated:</label>
                                    <span>${this.formatDate(form.updated_at)}</span>
                                </div>
                            </div>
                        </div>
                        
                        <div class="detail-section">
                            <h4>📊 Statistics</h4>
                            <div class="stats-grid">
                                <div class="stat-card">
                                    <div class="stat-value">${this.formatNumber(formStats.views || 0)}</div>
                                    <div class="stat-label">Total Views</div>
                                </div>
                                <div class="stat-card">
                                    <div class="stat-value">${this.formatNumber(formStats.submissions || 0)}</div>
                                    <div class="stat-label">Submissions</div>
                                </div>
                                <div class="stat-card">
                                    <div class="stat-value">${formStats.conversion_rate || '0'}%</div>
                                    <div class="stat-label">Conversion Rate</div>
                                </div>
                            </div>
                        </div>
                        
                        ${form.fields && form.fields.length > 0 ? `
                        <div class="detail-section">
                            <h4>📝 Form Fields</h4>
                            <div class="fields-list">
                                ${form.fields.map(field => `
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
            }
            
        } catch (error) {
            console.error('Error loading form details:', error);
            this.showToast('Failed to load form details', 'error');
        } finally {
            this.hideLoading();
        }
    }

    /**
     * Set button loading state
     */
    setButtonLoading(button, loading = true) {
        if (!button) return;
        
        const btnText = button.querySelector('.btn-text');
        const btnLoading = button.querySelector('.btn-loading');
        
        if (btnText && btnLoading) {
            if (loading) {
                btnText.style.display = 'none';
                btnLoading.style.display = 'inline';
                button.disabled = true;
            } else {
                btnText.style.display = 'inline';
                btnLoading.style.display = 'none';
                button.disabled = false;
            }
        } else {
            button.disabled = loading;
        }
    }

    /**
     * Update page title
     */
    updatePageTitle(title) {
        document.title = title ? `${title} - Leads Core Admin` : 'Leads Core Admin Panel';
    }

    /**
     * Animate element
     */
    animate(element, animation = 'fadeIn', duration = 300) {
        if (!element) return;
        
        element.style.animation = `${animation} ${duration}ms ease-out`;
        
        setTimeout(() => {
            element.style.animation = '';
        }, duration);
    }

    /**
     * Confirm action with user
     */
    confirm(message, callback) {
        if (window.confirm(message)) {
            callback();
        }
    }

    /**
     * Debounce function
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
