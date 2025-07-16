/**
 * Dashboard management module for Leads Core Admin Panel
 */

class DashboardManager {
    constructor() {
        this.currentPage = 1;
        this.perPage = 20;
        this.totalForms = 0;
        this.currentFilters = {
            search: '',
            status: '',
            type: ''
        };
        this.refreshInterval = null;
    }

    /**
     * Show login section
     */
    showLogin() {
        const loginSection = document.getElementById('login-section');
        const dashboardSection = document.getElementById('dashboard-section');
        
        if (loginSection) loginSection.style.display = 'flex';
        if (dashboardSection) dashboardSection.style.display = 'none';
        
        window.UI.updatePageTitle('Login');
    }

    /**
     * Show dashboard section
     */
    async showDashboard() {
        try {
            window.UI.showLoading();
            
            const loginSection = document.getElementById('login-section');
            const dashboardSection = document.getElementById('dashboard-section');
            
            if (loginSection) loginSection.style.display = 'none';
            if (dashboardSection) dashboardSection.style.display = 'block';
            
            // Update user info
            const user = window.AuthManager.getUser();
            const userInfoElement = document.getElementById('user-info');
            if (userInfoElement && user) {
                userInfoElement.textContent = `User: ${user.id}`;
            }
            
            // Load dashboard data
            await this.loadData();
            
            // Setup auto-refresh
            this.setupAutoRefresh();
            
            window.UI.updatePageTitle('Dashboard');
            
        } catch (error) {
            console.error('Error showing dashboard:', error);
            window.UI.showToast('Error loading dashboard: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Load all dashboard data
     */
    async loadData() {
        try {
            // Load summary data and forms in parallel
            await Promise.all([
                this.loadSummary(),
                this.loadForms()
            ]);
        } catch (error) {
            console.error('Error loading data:', error);
            window.UI.showToast('Error loading data: ' + error.message, 'error');
        }
    }

    /**
     * Load summary statistics
     */
    async loadSummary() {
        try {
            const summary = await window.APIClient.getFormsSummary();
            
            // Update summary cards
            this.updateSummaryCard('total-forms', summary.total_forms || 0);
            this.updateSummaryCard('active-forms', summary.active_forms || 0);
            this.updateSummaryCard('total-views', summary.total_views || 0);
            this.updateSummaryCard('total-submissions', summary.total_submissions || 0);
            
        } catch (error) {
            console.error('Error loading summary:', error);
            // Set default values on error
            this.updateSummaryCard('total-forms', 0);
            this.updateSummaryCard('active-forms', 0);
            this.updateSummaryCard('total-views', 0);
            this.updateSummaryCard('total-submissions', 0);
        }
    }

    /**
     * Update summary card value
     */
    updateSummaryCard(elementId, value) {
        const element = document.getElementById(elementId);
        if (element) {
            element.textContent = window.UI.formatNumber(value);
        }
    }

    /**
     * Load forms list
     */
    async loadForms() {
        try {
            const options = {
                page: this.currentPage,
                per_page: this.perPage,
                ...this.currentFilters
            };
            
            const response = await window.APIClient.getForms(options);
            this.totalForms = response.meta?.total || 0;
            
            this.renderFormsTable(response.data || []);
            this.updatePagination();
            this.populateFilterOptions(response.data || []);
            
        } catch (error) {
            console.error('Error loading forms:', error);
            this.renderFormsTable([]);
            this.updatePagination();
        }
    }

    /**
     * Render forms table
     */
    renderFormsTable(forms) {
        const tbody = document.getElementById('forms-tbody');
        if (!tbody) return;
        
        if (forms.length === 0) {
            tbody.innerHTML = `
                <tr class="no-data-row">
                    <td colspan="8">
                        <div class="no-data">
                            <div class="no-data-icon">📝</div>
                            <div class="no-data-text">No forms found</div>
                            <div class="no-data-subtext">
                                ${this.currentFilters.search || this.currentFilters.status || this.currentFilters.type 
                                    ? 'Try adjusting your search filters' 
                                    : 'Create your first form to get started'}
                            </div>
                        </div>
                    </td>
                </tr>
            `;
            return;
        }
        
        tbody.innerHTML = forms.map(form => `
            <tr class="form-row" data-form-id="${form.id}">
                <td class="form-id">${form.id}</td>
                <td class="form-name">
                    <div class="form-name-container">
                        <span class="name">${window.UI.escapeHtml(form.name || 'Untitled')}</span>
                        ${form.description ? `<span class="description">${window.UI.escapeHtml(form.description)}</span>` : ''}
                    </div>
                </td>
                <td class="form-type">
                    <span class="type-badge type-${form.type || 'other'}">${form.type || 'other'}</span>
                </td>
                <td class="form-status">
                    <span class="status-badge ${form.enabled ? 'enabled' : 'disabled'}">
                        ${form.enabled ? '✅ Active' : '❌ Disabled'}
                    </span>
                </td>
                <td class="form-created">${window.UI.formatDate(form.created_at)}</td>
                <td class="form-views">${window.UI.formatNumber(form.stats.views || 0)}</td>
                <td class="form-submissions">${window.UI.formatNumber(form.stats.submits || 0)}</td>
                <td class="form-actions">
                    <div class="table-actions">
                        <button class="btn-icon" onclick="window.UI.showFormDetails('${form.id}')" title="View Details">
                            👁️
                        </button>
                        <button class="btn-icon" onclick="window.FormsManager.editForm('${form.id}')" title="Edit Form">
                            ✏️
                        </button>
                        <button class="btn-icon" onclick="window.FormsManager.toggleFormStatus('${form.id}', ${!form.enabled})" title="${form.enabled ? 'Disable' : 'Enable'}">
                            ${form.enabled ? '⏸️' : '▶️'}
                        </button>
                        <button class="btn-icon btn-danger" onclick="window.FormsManager.deleteForm('${form.id}', '${window.UI.escapeHtml(form.name || 'Untitled')}')" title="Delete Form">
                            🗑️
                        </button>
                    </div>
                </td>
            </tr>
        `).join('');
    }

    /**
     * Update pagination controls
     */
    updatePagination() {
        const totalPages = Math.ceil(this.totalForms / this.perPage) || 1;
        
        // Update pagination info
        const paginationInfo = document.getElementById('pagination-info');
        const pageInfo = document.getElementById('page-info');
        const prevBtn = document.getElementById('prev-page');
        const nextBtn = document.getElementById('next-page');
        const paginationContainer = document.querySelector('.pagination-container');
        
        if (this.totalForms === 0) {
            if (paginationContainer) {
                paginationContainer.style.display = 'none';
            }
            return;
        }
        
        if (paginationContainer) {
            paginationContainer.style.display = 'flex';
        }
        
        const startItem = ((this.currentPage - 1) * this.perPage) + 1;
        const endItem = Math.min(this.currentPage * this.perPage, this.totalForms);
        
        if (paginationInfo) {
            paginationInfo.textContent = `Showing ${startItem}-${endItem} of ${this.totalForms} forms`;
        }
        
        if (pageInfo) {
            pageInfo.textContent = `Page ${this.currentPage} of ${totalPages}`;
        }
        
        if (prevBtn) {
            prevBtn.disabled = this.currentPage <= 1;
        }
        
        if (nextBtn) {
            nextBtn.disabled = this.currentPage >= totalPages;
        }
    }

    /**
     * Populate filter options
     */
    populateFilterOptions(forms) {
        const typeFilter = document.getElementById('filter-type');
        if (!typeFilter) return;
        
        // Get unique types
        const types = [...new Set(forms.map(form => form.type).filter(Boolean))];
        
        // Clear current options (except "All Types")
        typeFilter.innerHTML = '<option value="">All Types</option>';
        
        // Add type options
        types.forEach(type => {
            const option = document.createElement('option');
            option.value = type;
            option.textContent = type.charAt(0).toUpperCase() + type.slice(1);
            typeFilter.appendChild(option);
        });
    }

    /**
     * Handle search input
     */
    handleSearch(query) {
        this.currentFilters.search = query;
        this.currentPage = 1;
        this.loadForms();
    }

    /**
     * Handle filter change
     */
    handleFilter(filterType, value) {
        this.currentFilters[filterType] = value;
        this.currentPage = 1;
        this.loadForms();
    }

    /**
     * Go to previous page
     */
    previousPage() {
        if (this.currentPage > 1) {
            this.currentPage--;
            this.loadForms();
        }
    }

    /**
     * Go to next page
     */
    nextPage() {
        const totalPages = Math.ceil(this.totalForms / this.perPage);
        if (this.currentPage < totalPages) {
            this.currentPage++;
            this.loadForms();
        }
    }

    /**
     * Refresh all data
     */
    async refreshData() {
        try {
            window.UI.showLoading();
            await this.loadData();
            window.UI.showToast('Data refreshed successfully', 'success');
        } catch (error) {
            console.error('Error refreshing data:', error);
            window.UI.showToast('Error refreshing data: ' + error.message, 'error');
        } finally {
            window.UI.hideLoading();
        }
    }

    /**
     * Setup auto-refresh interval
     */
    setupAutoRefresh() {
        // Clear existing interval
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
        
        // Set up new interval (refresh every 30 seconds)
        this.refreshInterval = setInterval(() => {
            if (window.AuthManager.isAuthenticated()) {
                this.loadData();
            } else {
                this.clearAutoRefresh();
            }
        }, 30000);
    }

    /**
     * Clear auto-refresh interval
     */
    clearAutoRefresh() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
            this.refreshInterval = null;
        }
    }

    /**
     * Reset filters
     */
    resetFilters() {
        this.currentFilters = {
            search: '',
            status: '',
            type: ''
        };
        this.currentPage = 1;
        
        // Reset UI elements
        const searchInput = document.getElementById('search-input');
        const statusFilter = document.getElementById('filter-status');
        const typeFilter = document.getElementById('filter-type');
        
        if (searchInput) searchInput.value = '';
        if (statusFilter) statusFilter.value = '';
        if (typeFilter) typeFilter.value = '';
        
        this.loadForms();
    }
}

// Create global instance
window.Dashboard = new DashboardManager();
