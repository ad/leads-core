/**
 * Widgets management module for Leads Core Admin Panel
 */

class WidgetsManager {
    constructor() {
        this.currentWidgetId = null;
        this.widgetToDelete = null;
        this.widgetFields = [];
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
            this.clearWidgetFields();
            this.setupWidgetEditorEvents();
        }
    }

    /**
     * Show edit widget modal
     */
    async editWidget(widgetId) {
        try {
            window.UI.showLoading();
            const widget = await window.APIClient.getWidget(widgetId);
            

            
            const modal = document.getElementById('widget-editor-modal');
            const title = document.getElementById('widget-editor-title');
            
            if (modal && title) {
                modal.style.display = 'flex';
                title.textContent = 'Edit Widget';
                
                // Populate widget data
                const nameInput = document.getElementById('widget-name');
                const typeSelect = document.getElementById('widget-type');
                const enabledCheckbox = document.getElementById('widget-enabled');
                
                if (nameInput) nameInput.value = widget.name || '';
                if (typeSelect) typeSelect.value = widget.type || '';
                if (enabledCheckbox) enabledCheckbox.checked = widget.enabled || false;
                
                console.log('Widget basic data populated:', {
                    name: widget.name,
                    type: widget.type,
                    enabled: widget.enabled
                });
                
                this.currentWidgetId = widgetId;
                this.loadWidgetFields(widget.fields || {});
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
            addFieldBtn.onclick = () => this.addWidgetField();
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
     * Clear widget fields container
     */
    clearWidgetFields() {
        const container = document.getElementById('widget-fields-container');
        if (container) {
            container.innerHTML = '';
        }
        this.widgetFields = [];
    }

    /**
     * Load widget fields into editor
     */
    loadWidgetFields(fields) {

        this.clearWidgetFields();
        this.widgetFields = [];
        
        // Convert object fields to array format for the UI
        if (fields && typeof fields === 'object') {
            console.log('Processing fields:', Object.keys(fields));
            Object.entries(fields).forEach(([fieldName, fieldData]) => {
                const field = {
                    id: `field_${Date.now()}_${Math.random()}`,
                    name: fieldName,
                    type: fieldData.type || 'text',
                    required: fieldData.required || false,
                    placeholder: fieldData.placeholder || '',
                    description: fieldData.description || ''
                };
                console.log('Adding field:', field);
                this.widgetFields.push(field);
                this.addWidgetField(field);
            });
        } else {
            console.log('No fields to load or invalid fields format');
        }
        console.log('Final widgetFields array:', this.widgetFields);
    }

    /**
     * Add widget field to editor
     */
    addWidgetField(field = null) {
        const container = document.getElementById('widget-fields-container');
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
                    <button type="button" class="btn-icon btn-danger" onclick="window.WidgetsManager.removeWidgetField('${fieldId}')">🗑️</button>
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
     * Remove widget field
     */
    removeWidgetField(fieldId) {
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
                // Update existing widget
                await window.APIClient.updateWidget(this.currentWidgetId, widgetData);
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
        const enabledCheckbox = document.getElementById('widget-enabled');
        
        const name = nameInput?.value?.trim();
        const type = typeSelect?.value;
        const enabled = enabledCheckbox?.checked || false;
        
        if (!name) {
            window.UI.showToast('Widget name is required', 'error');
            return null;
        }
        
        if (!type) {
            window.UI.showToast('Widget type is required', 'error');
            return null;
        }
        
        const fields = {};
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
            
            fields[fieldName] = field;
        });
        
        return {
            name,
            type,
            enabled,
            fields
        };
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
        this.widgetFields = [];
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
    async toggleWidgetStatus(widgetId, enabled) {
        try {
            window.UI.showLoading();
            
            // Get current widget data
            const widget = await window.APIClient.getWidget(widgetId);
            
            // Update only the enabled status
            const updateData = {
                ...widget,
                enabled: enabled
            };
            
            await window.APIClient.updateWidget(widgetId, updateData);
            window.UI.showToast(`Widget ${enabled ? 'enabled' : 'disabled'} successfully`, 'success');
            
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
}

// Create global instance
window.WidgetsManager = new WidgetsManager();
