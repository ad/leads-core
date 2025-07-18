/**
 * Widgets management module for Leads Core Admin Panel
 */

class WidgetsManager {
    constructor() {
        this.currentWidgetId = null;
        this.widgetToDelete = null;
        this.widgetConfig = [];
    }

    /**
     * Show create widget modal
     */
    showCreateWidget() {
        const modal = document.getElementById('widget-editor-modal');
        const title = document.getElementById('widget-editor-title');
        const widget = document.getElementById('widget-editor-widget');
        
        if (modal && title && widget) {
            modal.style.display = 'flex';
            title.textContent = 'Create New Widget';
            widget.reset();
            this.currentWidgetId = null;
            this.clearWidgetConfig();
            this.setupWidgetEditorEvents();
        }
    }

    /**
     * Show edit widget modal
     */
    async editWidget(widgetId) {
        try {
            window.UI.showLoading();
            console.log('Loading widget with ID:', widgetId);
            const widget = await window.APIClient.getWidget(widgetId);
            console.log('Loaded widget:', widget);
            const modal = document.getElementById('widget-editor-modal');
            const title = document.getElementById('widget-editor-title');
            
            if (modal && title) {
                modal.style.display = 'flex';
                title.textContent = 'Edit Widget';
                
                // Populate widget data
                const nameInput = document.getElementById('widget-name');
                const typeSelect = document.getElementById('widget-type');
                const visibleCheckbox = document.getElementById('widget-visible');
                
                if (nameInput) nameInput.value = widget.name || '';
                if (typeSelect) typeSelect.value = widget.type || '';
                if (visibleCheckbox) visibleCheckbox.checked = widget.isVisible || false;
                
                console.log('Widget basic data populated:', {
                    name: widget.name,
                    type: widget.type,
                    isVisible: widget.isVisible
                });
                
                this.currentWidgetId = widgetId;
                this.loadWidgetConfig(widget.config || {});
                this.setupWidgetEditorEvents();
            }
            
        } catch (error) {
            console.error('Error loading widget:', error);
            window.UI.showToast('Failed to load widget', 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Setup widget editor event listeners
     */
    setupWidgetEditorEvents() {
        // Add field button
        const addFieldBtn = document.getElementById('add-field-btn');
        if (addFieldBtn) {
            addFieldBtn.onclick = () => this.addWidgetConfigField();
        }

        // Save widget button
        const saveWidgetBtn = document.getElementById('widget-editor-save');
        if (saveWidgetBtn) {
            saveWidgetBtn.onclick = (e) => {
                e.preventDefault();
                this.saveWidget();
            };
        }

        // Cancel widget button
        const cancelWidgetBtn = document.getElementById('widget-editor-cancel');
        if (cancelWidgetBtn) {
            cancelWidgetBtn.onclick = () => this.closeWidgetEditor();
        }

        // Close modal button
        const closeBtn = document.getElementById('widget-editor-close');
        if (closeBtn) {
            closeBtn.onclick = () => this.closeWidgetEditor();
        }

        // Widget submission
        const widget = document.getElementById('widget-editor-widget');
        if (widget) {
            widget.onsubmit = (e) => {
                e.preventDefault();
                this.saveWidget();
            };
        }
    }

    /**
     * Clear widget config container
     */
    clearWidgetConfig() {
        const container = document.getElementById('widget-config-container');
        if (container) {
            container.innerHTML = '';
        }
        this.widgetConfig = [];
    }

    /**
     * Load widget config into editor
     */
    loadWidgetConfig(config) {

        this.clearWidgetConfig();
        this.widgetConfig = [];
        
        // Convert object config to array format for the UI
        if (config && typeof config === 'object') {
            console.log('Processing config:', Object.keys(config));
            Object.entries(config).forEach(([fieldName, fieldData]) => {
                const field = {
                    id: `field_${Date.now()}_${Math.random()}`,
                    name: fieldName,
                    type: fieldData.type || 'text',
                    required: fieldData.required || false,
                    placeholder: fieldData.placeholder || '',
                    description: fieldData.description || ''
                };
                console.log('Adding field:', field);
                this.widgetConfig.push(field);
                this.addWidgetConfigField(field);
            });
        } else {
            console.log('No config to load or invalid config format');
        }
        console.log('Final widgetConfig array:', this.widgetConfig);
    }

    /**
     * Add widget config field to editor
     */
    addWidgetConfigField(field = null) {
        const container = document.getElementById('widget-config-container');
        if (!container) return;
        
        const fieldId = field ? field.id : `field_${Date.now()}`;
        
        const fieldHtml = `
            <div class="widget-field-item" data-field-id="${fieldId}">
                <div class="widget-field-header">
                    <input type="text" class="field-name" placeholder="Field Name" value="${field ? field.name || '' : ''}" />
                    <select class="field-type">
                        <option value="text" ${field && field.type === 'text' ? 'selected' : ''}>Text</option>
                        <option value="email" ${field && field.type === 'email' ? 'selected' : ''}>Email</option>
                        <option value="tel" ${field && field.type === 'tel' ? 'selected' : ''}>Phone</option>
                        <option value="number" ${field && field.type === 'number' ? 'selected' : ''}>Number</option>
                        <option value="textarea" ${field && field.type === 'textarea' ? 'selected' : ''}>Textarea</option>
                        <option value="select" ${field && field.type === 'select' ? 'selected' : ''}>Select</option>
                        <option value="checkbox" ${field && field.type === 'checkbox' ? 'selected' : ''}>Checkbox</option>
                        <option value="radio" ${field && field.type === 'radio' ? 'selected' : ''}>Radio</option>
                        <option value="file" ${field && field.type === 'file' ? 'selected' : ''}>File Upload</option>
                    </select>
                    <label class="field-required">
                        <input type="checkbox" ${field && field.required ? 'checked' : ''} />
                        Required
                    </label>
                    <button type="button" class="btn-icon btn-danger" onclick="window.WidgetsManager.removeWidgetConfigField('${fieldId}')">🗑️</button>
                </div>
                <div class="widget-field-details">
                    <input type="text" class="field-placeholder" placeholder="Placeholder text" value="${field ? field.placeholder || '' : ''}" />
                    <textarea class="field-description" placeholder="Field description" rows="2">${field ? field.description || '' : ''}</textarea>
                </div>
            </div>
        `;
        
        container.insertAdjacentHTML('beforeend', fieldHtml);
    }

    /**
     * Remove widget config field
     */
    removeWidgetConfigField(fieldId) {
        const fieldElement = document.querySelector(`[data-field-id="${fieldId}"]`);
        if (fieldElement) {
            fieldElement.remove();
        }
    }

    /**
     * Save widget (create or update)
     */
    async saveWidget() {
        try {
            const widgetData = this.getWidgetData();
            if (!widgetData) return;

            window.UI.showLoading();
            
            if (this.currentWidgetId) {
                // Update existing widget metadata and config separately
                const metadataUpdate = {
                    name: widgetData.name,
                    type: widgetData.type,
                    isVisible: widgetData.isVisible
                };
                
                // Update widget metadata
                await window.APIClient.updateWidget(this.currentWidgetId, metadataUpdate);
                
                // Update widget config separately
                if (widgetData.config && Object.keys(widgetData.config).length > 0) {
                    await window.APIClient.updateWidgetConfig(this.currentWidgetId, { config: widgetData.config });
                }
                
                window.UI.showToast('Widget updated successfully', 'success');
            } else {
                // Create new widget
                await window.APIClient.createWidget(widgetData);
                window.UI.showToast('Widget created successfully', 'success');
            }
            
            this.closeWidgetEditor();
            
            // Refresh widgets list
            if (window.Dashboard) {
                window.Dashboard.loadWidgets();
            }
            
        } catch (error) {
            console.error('Error saving widget:', error);
            window.UI.showToast('Failed to save widget: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Get widget data from editor
     */
    getWidgetData() {
        const nameInput = document.getElementById('widget-name');
        const typeSelect = document.getElementById('widget-type');
        const visibleCheckbox = document.getElementById('widget-visible');
        
        const name = nameInput?.value?.trim();
        const type = typeSelect?.value;
        const isVisible = visibleCheckbox?.checked || false;
        
        if (!name) {
            window.UI.showToast('Widget name is required', 'error');
            return null;
        }
        
        if (!type) {
            window.UI.showToast('Widget type is required', 'error');
            return null;
        }
        
        const config = {};
        const fieldElements = document.querySelectorAll('.widget-field-item');
        
        fieldElements.forEach(element => {
            const fieldName = element.querySelector('.field-name').value.trim();
            if (!fieldName) return;
            
            const field = {
                type: element.querySelector('.field-type').value,
                required: element.querySelector('.field-required input').checked
            };
            
            const placeholder = element.querySelector('.field-placeholder').value.trim();
            const description = element.querySelector('.field-description').value.trim();
            
            if (placeholder) field.placeholder = placeholder;
            if (description) field.description = description;
            
            config[fieldName] = field;
        });
        
        return {
            name,
            type,
            isVisible,
            config
        };
    }

    /**
     * Update widget configuration only
     */
    async updateWidgetConfig(widgetId, config) {
        try {
            window.UI.showLoading();
            await window.APIClient.updateWidgetConfig(widgetId, { config });
            window.UI.showToast('Widget configuration updated successfully', 'success');
            
            // Refresh widgets list
            if (window.Dashboard) {
                window.Dashboard.loadWidgets();
            }
        } catch (error) {
            console.error('Error updating widget config:', error);
            window.UI.showToast('Failed to update widget configuration: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Close widget editor modal
     */
    closeWidgetEditor() {
        const modal = document.getElementById('widget-editor-modal');
        if (modal) {
            modal.style.display = 'none';
        }
        this.currentWidgetId = null;
        this.widgetConfig = [];
    }

    /**
     * Show delete confirmation
     */
    deleteWidget(widgetId, widgetName) {
        this.widgetToDelete = widgetId;
        
        const deleteWidgetNameElement = document.getElementById('delete-widget-name');
        if (deleteWidgetNameElement) {
            deleteWidgetNameElement.textContent = widgetName;
        }
        
        const modal = document.getElementById('delete-confirm-modal');
        if (modal) {
            modal.style.display = 'flex';
        }

        // Setup delete confirmation events
        this.setupDeleteConfirmEvents();
    }

    /**
     * Setup delete confirmation events
     */
    setupDeleteConfirmEvents() {
        // Confirm delete button
        const confirmDeleteBtn = document.getElementById('delete-confirm');
        if (confirmDeleteBtn) {
            confirmDeleteBtn.onclick = () => this.confirmDelete();
        }

        // Cancel delete button
        const cancelDeleteBtn = document.getElementById('delete-cancel');
        if (cancelDeleteBtn) {
            cancelDeleteBtn.onclick = () => this.closeDeleteConfirm();
        }

        // Close modal button
        const closeBtn = document.getElementById('delete-confirm-close');
        if (closeBtn) {
            closeBtn.onclick = () => this.closeDeleteConfirm();
        }
    }

    /**
     * Confirm widget deletion
     */
    async confirmDelete() {
        if (!this.widgetToDelete) return;
        
        try {
            window.UI.showLoading();
            await window.APIClient.deleteWidget(this.widgetToDelete);
            window.UI.showToast('Widget deleted successfully', 'success');
            this.closeDeleteConfirm();
            
            // Refresh widgets list
            if (window.Dashboard) {
                window.Dashboard.loadWidgets();
            }
        } catch (error) {
            console.error('Error deleting widget:', error);
            window.UI.showToast('Failed to delete widget: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Close delete confirmation modal
     */
    closeDeleteConfirm() {
        const modal = document.getElementById('delete-confirm-modal');
        if (modal) {
            modal.style.display = 'none';
        }
        this.widgetToDelete = null;
    }

    /**
     * Toggle widget status (enable/disable)
     */
    async toggleWidgetStatus(widgetId, isVisible) {
        try {
            window.UI.showLoading();
            
            // Update only the isVisible status without fetching the full widget
            const updateData = {
                isVisible: isVisible
            };
            
            await window.APIClient.updateWidget(widgetId, updateData);
            window.UI.showToast(`Widget ${isVisible ? 'enabled' : 'disabled'} successfully`, 'success');

            // Refresh widgets list
            if (window.Dashboard) {
                window.Dashboard.loadWidgets();
            }
        } catch (error) {
            console.error('Error updating widget status:', error);
            window.UI.showToast('Failed to update widget status: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Export widget submissions
     */
    async exportSubmissions(widgetId, format = 'csv', dateRange = null) {
        try {
            window.UI.showLoading();
            
            let url = `/api/v1/widgets/${widgetId}/export?format=${format}`;
            
            // Add date range parameters if provided
            if (dateRange && dateRange.from) {
                url += `&from=${dateRange.from}`;
            }
            if (dateRange && dateRange.to) {
                url += `&to=${dateRange.to}`;
            }
            
            const response = await fetch(url, {
                headers: {
                    'Authorization': `Bearer ${window.AuthManager.getToken()}`
                }
            });
            
            if (!response.ok) {
                throw new Error(`Export failed: ${response.statusText}`);
            }
            
            // Get filename from response header
            const contentDisposition = response.headers.get('Content-Disposition');
            let filename = `submissions_${widgetId}.${format}`;
            if (contentDisposition) {
                const filenameMatch = contentDisposition.match(/filename="?([^"]+)"?/);
                if (filenameMatch) {
                    filename = filenameMatch[1];
                }
            }
            
            // Create blob and download
            const blob = await response.blob();
            const downloadUrl = window.URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = downloadUrl;
            link.download = filename;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            window.URL.revokeObjectURL(downloadUrl);
            
            window.UI.hideLoading();
            window.UI.showToast(`Submissions exported as ${format.toUpperCase()}`, 'success');
            
        } catch (error) {
            console.error('Error exporting submissions:', error);
            window.UI.hideLoading();
            window.UI.showToast('Failed to export submissions: ' + error.message, 'error');
        }
    }

    /**
     * Show export options modal
     */
    showExportModal(widgetId) {
        const modal = document.getElementById('export-modal');
        if (!modal) {
            this.createExportModal();
        }
        
        const exportModal = document.getElementById('export-modal');
        const widgetIdInput = document.getElementById('export-widget-id');
        
        if (exportModal && widgetIdInput) {
            widgetIdInput.value = widgetId;
            exportModal.style.display = 'flex';
        }
    }

    /**
     * Create export modal if it doesn't exist
     */
    createExportModal() {
        const modal = document.createElement('div');
        modal.id = 'export-modal';
        modal.className = 'modal';
        modal.innerHTML = `
            <div class="modal-content">
                <div class="modal-header">
                    <h3>Export Submissions</h3>
                    <span class="close" onclick="window.WidgetsManager.closeExportModal()">&times;</span>
                </div>
                <div class="modal-body">
                    <input type="hidden" id="export-widget-id">
                    
                    <div class="form-group">
                        <label for="export-format">Format:</label>
                        <select id="export-format" class="form-control">
                            <option value="csv">CSV</option>
                            <option value="json">JSON</option>
                            <option value="xlsx">Excel (XLSX)</option>
                        </select>
                    </div>
                    
                    <div class="form-group">
                        <label for="export-date-range">Date Range (optional):</label>
                        <div class="date-range-container">
                            <input type="datetime-local" id="export-date-from" class="form-control" placeholder="From">
                            <span>to</span>
                            <input type="datetime-local" id="export-date-to" class="form-control" placeholder="To">
                        </div>
                    </div>
                    
                    <div class="form-group">
                        <button type="button" class="btn btn-secondary" onclick="window.WidgetsManager.setQuickDateRange('today')">Today</button>
                        <button type="button" class="btn btn-secondary" onclick="window.WidgetsManager.setQuickDateRange('week')">This Week</button>
                        <button type="button" class="btn btn-secondary" onclick="window.WidgetsManager.setQuickDateRange('month')">This Month</button>
                        <button type="button" class="btn btn-secondary" onclick="window.WidgetsManager.setQuickDateRange('all')">All Time</button>
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" onclick="window.WidgetsManager.closeExportModal()">Cancel</button>
                    <button type="button" class="btn btn-primary" onclick="window.WidgetsManager.startExport()">Export</button>
                </div>
            </div>
        `;
        
        document.body.appendChild(modal);
        
        // Add some basic styling
        const style = document.createElement('style');
        style.textContent = `
            .date-range-container {
                display: flex;
                align-items: center;
                gap: 10px;
            }
            .date-range-container input {
                flex: 1;
            }
            .date-range-container span {
                color: #666;
                font-weight: bold;
            }
        `;
        document.head.appendChild(style);
    }

    /**
     * Set quick date range
     */
    setQuickDateRange(range) {
        const fromInput = document.getElementById('export-date-from');
        const toInput = document.getElementById('export-date-to');
        
        if (!fromInput || !toInput) return;
        
        const now = new Date();
        let from, to;
        
        switch (range) {
            case 'today':
                from = new Date(now.getFullYear(), now.getMonth(), now.getDate());
                to = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 23, 59, 59);
                break;
            case 'week':
                const startOfWeek = new Date(now);
                startOfWeek.setDate(now.getDate() - now.getDay());
                startOfWeek.setHours(0, 0, 0, 0);
                from = startOfWeek;
                to = now;
                break;
            case 'month':
                from = new Date(now.getFullYear(), now.getMonth(), 1);
                to = now;
                break;
            case 'all':
                fromInput.value = '';
                toInput.value = '';
                return;
        }
        
        // Convert to local datetime string format
        fromInput.value = from.toISOString().slice(0, 16);
        toInput.value = to.toISOString().slice(0, 16);
    }

    /**
     * Start export process
     */
    async startExport() {
        const widgetId = document.getElementById('export-widget-id').value;
        const format = document.getElementById('export-format').value;
        const dateFrom = document.getElementById('export-date-from').value;
        const dateTo = document.getElementById('export-date-to').value;
        
        let dateRange = null;
        if (dateFrom || dateTo) {
            dateRange = {};
            if (dateFrom) dateRange.from = new Date(dateFrom).toISOString();
            if (dateTo) dateRange.to = new Date(dateTo).toISOString();
        }
        
        this.closeExportModal();
        await this.exportSubmissions(widgetId, format, dateRange);
    }

    /**
     * Close export modal
     */
    closeExportModal() {
        const modal = document.getElementById('export-modal');
        if (modal) {
            modal.style.display = 'none';
        }
    }
}

// Create global instance
const widgetsManagerInstance = new WidgetsManager();
window.WidgetsManager = widgetsManagerInstance;

// Make specific methods globally accessible
window.WidgetsManager.removeWidgetConfigField = (fieldId) => widgetsManagerInstance.removeWidgetConfigField(fieldId);
