/**
 * Forms management module for Leads Core Admin Panel
 */

class FormsManager {
    constructor() {
        this.currentFormId = null;
        this.formToDelete = null;
        this.formFields = [];
    }

    /**
     * Show create form modal
     */
    showCreateForm() {
        const modal = document.getElementById('form-editor-modal');
        const title = document.getElementById('form-editor-title');
        const form = document.getElementById('form-editor-form');
        
        if (modal && title && form) {
            modal.style.display = 'flex';
            title.textContent = 'Create New Form';
            form.reset();
            this.currentFormId = null;
            this.clearFormFields();
            this.setupFormEditorEvents();
        }
    }

    /**
     * Show edit form modal
     */
    async editForm(formId) {
        try {
            window.UI.showLoading();
            const form = await window.APIClient.getForm(formId);
            

            
            const modal = document.getElementById('form-editor-modal');
            const title = document.getElementById('form-editor-title');
            
            if (modal && title) {
                modal.style.display = 'flex';
                title.textContent = 'Edit Form';
                
                // Populate form data
                const nameInput = document.getElementById('form-name');
                const typeSelect = document.getElementById('form-type');
                const enabledCheckbox = document.getElementById('form-enabled');
                
                if (nameInput) nameInput.value = form.name || '';
                if (typeSelect) typeSelect.value = form.type || '';
                if (enabledCheckbox) enabledCheckbox.checked = form.enabled || false;
                
                console.log('Form basic data populated:', {
                    name: form.name,
                    type: form.type,
                    enabled: form.enabled
                });
                
                this.currentFormId = formId;
                this.loadFormFields(form.fields || {});
                this.setupFormEditorEvents();
            }
            
        } catch (error) {
            console.error('Error loading form:', error);
            window.UI.showToast('Failed to load form', 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Setup form editor event listeners
     */
    setupFormEditorEvents() {
        // Add field button
        const addFieldBtn = document.getElementById('add-field-btn');
        if (addFieldBtn) {
            addFieldBtn.onclick = () => this.addFormField();
        }

        // Save form button
        const saveFormBtn = document.getElementById('form-editor-save');
        if (saveFormBtn) {
            saveFormBtn.onclick = (e) => {
                e.preventDefault();
                this.saveForm();
            };
        }

        // Cancel form button
        const cancelFormBtn = document.getElementById('form-editor-cancel');
        if (cancelFormBtn) {
            cancelFormBtn.onclick = () => this.closeFormEditor();
        }

        // Close modal button
        const closeBtn = document.getElementById('form-editor-close');
        if (closeBtn) {
            closeBtn.onclick = () => this.closeFormEditor();
        }

        // Form submission
        const form = document.getElementById('form-editor-form');
        if (form) {
            form.onsubmit = (e) => {
                e.preventDefault();
                this.saveForm();
            };
        }
    }

    /**
     * Clear form fields container
     */
    clearFormFields() {
        const container = document.getElementById('form-fields-container');
        if (container) {
            container.innerHTML = '';
        }
        this.formFields = [];
    }

    /**
     * Load form fields into editor
     */
    loadFormFields(fields) {

        this.clearFormFields();
        this.formFields = [];
        
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
                this.formFields.push(field);
                this.addFormField(field);
            });
        } else {
            console.log('No fields to load or invalid fields format');
        }
        console.log('Final formFields array:', this.formFields);
    }

    /**
     * Add form field to editor
     */
    addFormField(field = null) {
        const container = document.getElementById('form-fields-container');
        if (!container) return;
        
        const fieldId = field ? field.id : `field_${Date.now()}`;
        
        const fieldHtml = `
            <div class="form-field-item" data-field-id="${fieldId}">
                <div class="form-field-header">
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
                    <button type="button" class="btn-icon btn-danger" onclick="window.FormsManager.removeFormField('${fieldId}')">🗑️</button>
                </div>
                <div class="form-field-details">
                    <input type="text" class="field-placeholder" placeholder="Placeholder text" value="${field ? field.placeholder || '' : ''}" />
                    <textarea class="field-description" placeholder="Field description" rows="2">${field ? field.description || '' : ''}</textarea>
                </div>
            </div>
        `;
        
        container.insertAdjacentHTML('beforeend', fieldHtml);
    }

    /**
     * Remove form field
     */
    removeFormField(fieldId) {
        const fieldElement = document.querySelector(`[data-field-id="${fieldId}"]`);
        if (fieldElement) {
            fieldElement.remove();
        }
    }

    /**
     * Save form (create or update)
     */
    async saveForm() {
        try {
            const formData = this.getFormData();
            if (!formData) return;

            window.UI.showLoading();
            
            if (this.currentFormId) {
                // Update existing form
                await window.APIClient.updateForm(this.currentFormId, formData);
                window.UI.showToast('Form updated successfully', 'success');
            } else {
                // Create new form
                await window.APIClient.createForm(formData);
                window.UI.showToast('Form created successfully', 'success');
            }
            
            this.closeFormEditor();
            
            // Refresh forms list
            if (window.Dashboard) {
                window.Dashboard.loadForms();
            }
            
        } catch (error) {
            console.error('Error saving form:', error);
            window.UI.showToast('Failed to save form: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Get form data from editor
     */
    getFormData() {
        const nameInput = document.getElementById('form-name');
        const typeSelect = document.getElementById('form-type');
        const enabledCheckbox = document.getElementById('form-enabled');
        
        const name = nameInput?.value?.trim();
        const type = typeSelect?.value;
        const enabled = enabledCheckbox?.checked || false;
        
        if (!name) {
            window.UI.showToast('Form name is required', 'error');
            return null;
        }
        
        if (!type) {
            window.UI.showToast('Form type is required', 'error');
            return null;
        }
        
        const fields = {};
        const fieldElements = document.querySelectorAll('.form-field-item');
        
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
     * Close form editor modal
     */
    closeFormEditor() {
        const modal = document.getElementById('form-editor-modal');
        if (modal) {
            modal.style.display = 'none';
        }
        this.currentFormId = null;
        this.formFields = [];
    }

    /**
     * Show delete confirmation
     */
    deleteForm(formId, formName) {
        this.formToDelete = formId;
        
        const deleteFormNameElement = document.getElementById('delete-form-name');
        if (deleteFormNameElement) {
            deleteFormNameElement.textContent = formName;
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
     * Confirm form deletion
     */
    async confirmDelete() {
        if (!this.formToDelete) return;
        
        try {
            window.UI.showLoading();
            await window.APIClient.deleteForm(this.formToDelete);
            window.UI.showToast('Form deleted successfully', 'success');
            this.closeDeleteConfirm();
            
            // Refresh forms list
            if (window.Dashboard) {
                window.Dashboard.loadForms();
            }
        } catch (error) {
            console.error('Error deleting form:', error);
            window.UI.showToast('Failed to delete form: ' + error.message, 'error');
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
        this.formToDelete = null;
    }

    /**
     * Toggle form status (enable/disable)
     */
    async toggleFormStatus(formId, enabled) {
        try {
            window.UI.showLoading();
            
            // Get current form data
            const form = await window.APIClient.getForm(formId);
            
            // Update only the enabled status
            const updateData = {
                ...form,
                enabled: enabled
            };
            
            await window.APIClient.updateForm(formId, updateData);
            window.UI.showToast(`Form ${enabled ? 'enabled' : 'disabled'} successfully`, 'success');
            
            // Refresh forms list
            if (window.Dashboard) {
                window.Dashboard.loadForms();
            }
        } catch (error) {
            console.error('Error updating form status:', error);
            window.UI.showToast('Failed to update form status: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }
}

// Create global instance
window.FormsManager = new FormsManager();
