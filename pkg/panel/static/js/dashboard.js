/**
 * Dashboard management module for Leads Core Admin Panel
 */

class DashboardManager {
    constructor() {
        this.currentPage = 1;
        this.perPage = 20;
        this.totalWidgets = 0;
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
            // Load summary data and widgets in parallel
            await Promise.all([
                this.loadSummary(),
                this.loadWidgets()
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
            const summary = await window.APIClient.getWidgetsSummary();
            
            // Update summary cards
            this.updateSummaryCard('total-widgets', summary.total_widgets || 0);
            this.updateSummaryCard('active-widgets', summary.active_widgets || 0);
            this.updateSummaryCard('total-views', summary.total_views || 0);
            this.updateSummaryCard('total-submissions', summary.total_submissions || 0);
            
        } catch (error) {
            console.error('Error loading summary:', error);
            // Set default values on error
            this.updateSummaryCard('total-widgets', 0);
            this.updateSummaryCard('active-widgets', 0);
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
     * Load widgets list
     */
    async loadWidgets() {
        try {
            const options = {
                page: this.currentPage,
                per_page: this.perPage,
                ...this.currentFilters
            };
            
            const response = await window.APIClient.getWidgets(options);
            this.totalWidgets = response.meta?.total || 0;
            
            this.renderWidgetsTable(response.data || []);
            this.updatePagination();
            this.populateFilterOptions(response.data || []);
            
        } catch (error) {
            console.error('Error loading widgets:', error);
            this.renderWidgetsTable([]);
            this.updatePagination();
        }
    }

    /**
     * Render widgets table
     */
    renderWidgetsTable(widgets) {
        const tbody = document.getElementById('widgets-tbody');
        if (!tbody) return;
        
        if (widgets.length === 0) {
            tbody.innerHTML = `
                <tr class="no-data-row">
                    <td colspan="8">
                        <div class="no-data">
                            <div class="no-data-icon">📝</div>
                            <div class="no-data-text">No widgets found</div>
                            <div class="no-data-subtext">
                                ${this.currentFilters.search || this.currentFilters.status || this.currentFilters.type 
                                    ? 'Try adjusting your search filters' 
                                    : 'Create your first widget to get started'}
                            </div>
                        </div>
                    </td>
                </tr>
            `;
            return;
        }
        
        tbody.innerHTML = widgets.map(widget => `
            <tr class="widget-row" data-widget-id="${widget.id}">
                <td class="widget-id">${widget.id}</td>
                <td class="widget-name">
                    <div class="widget-name-container">
                        <span class="name">${window.UI.escapeHtml(widget.name || 'Untitled')}</span>
                        ${widget.description ? `<span class="description">${window.UI.escapeHtml(widget.description)}</span>` : ''}
                    </div>
                </td>
                <td class="widget-type">
                    <span class="type-badge type-${widget.type || 'other'}">${widget.type || 'other'}</span>
                </td>
                <td class="widget-status">
                    <span class="status-badge ${widget.enabled ? 'enabled' : 'disabled'}">
                        ${widget.enabled ? '✅ Active' : '❌ Disabled'}
                    </span>
                </td>
                <td class="widget-created">${window.UI.formatDate(widget.created_at)}</td>
                <td class="widget-views">${window.UI.formatNumber(widget.stats.views || 0)}</td>
                <td class="widget-submissions">${window.UI.formatNumber(widget.stats.submits || 0)}</td>
                <td class="widget-actions">
                    <div class="table-actions">
                        <button class="btn-icon" onclick="window.UI.showWidgetDetails('${widget.id}')" title="View Details">
                            👁️
                        </button>
                        <button class="btn-icon" onclick="window.WidgetsManager.editWidget('${widget.id}')" title="Edit Widget">
                            ✏️
                        </button>
                        <button class="btn-icon" onclick="window.WidgetsManager.showExportModal('${widget.id}')" title="Export Submissions">
                            📥
                        </button>
                        <button class="btn-icon" onclick="window.WidgetsManager.toggleWidgetStatus('${widget.id}', ${!widget.enabled})" title="${widget.enabled ? 'Disable' : 'Enable'}">
                            ${widget.enabled ? '⏸️' : '▶️'}
                        </button>
                        <button class="btn-icon btn-danger" onclick="window.WidgetsManager.deleteWidget('${widget.id}', '${window.UI.escapeHtml(widget.name || 'Untitled')}')" title="Delete Widget">
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
        const totalPages = Math.ceil(this.totalWidgets / this.perPage) || 1;
        
        // Update pagination info
        const paginationInfo = document.getElementById('pagination-info');
        const pageInfo = document.getElementById('page-info');
        const prevBtn = document.getElementById('prev-page');
        const nextBtn = document.getElementById('next-page');
        const paginationContainer = document.querySelector('.pagination-container');
        
        if (this.totalWidgets === 0) {
            if (paginationContainer) {
                paginationContainer.style.display = 'none';
            }
            return;
        }
        
        if (paginationContainer) {
            paginationContainer.style.display = 'flex';
        }
        
        const startItem = ((this.currentPage - 1) * this.perPage) + 1;
        const endItem = Math.min(this.currentPage * this.perPage, this.totalWidgets);
        
        if (paginationInfo) {
            paginationInfo.textContent = `Showing ${startItem}-${endItem} of ${this.totalWidgets} widgets`;
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
    populateFilterOptions(widgets) {
        const typeFilter = document.getElementById('filter-type');
        if (!typeFilter) return;
        
        // Get unique types
        const types = [...new Set(widgets.map(widget => widget.type).filter(Boolean))];
        
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
        this.loadWidgets();
    }

    /**
     * Handle filter change
     */
    handleFilter(filterType, value) {
        this.currentFilters[filterType] = value;
        this.currentPage = 1;
        this.loadWidgets();
    }

    /**
     * Go to previous page
     */
    previousPage() {
        if (this.currentPage > 1) {
            this.currentPage--;
            this.loadWidgets();
        }
    }

    /**
     * Go to next page
     */
    nextPage() {
        const totalPages = Math.ceil(this.totalWidgets / this.perPage);
        if (this.currentPage < totalPages) {
            this.currentPage++;
            this.loadWidgets();
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
        
        this.loadWidgets();
    }
}

// Create global instance
window.Dashboard = new DashboardManager();
