/**
 * Main application controller for Leads Core Admin Panel
 */

class AdminPanel {
    constructor() {
        // Initialize after DOM is loaded
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => this.init());
        } else {
            this.init();
        }
    }

    /**
     * Initialize the application
     */
    async init() {
        this.bindEvents();
        window.UI.hideLoading();
        
        // Check if already authenticated
        if (window.AuthManager.isAuthenticated()) {
            await window.Dashboard.showDashboard();
        } else {
            window.Dashboard.showLogin();
        }
    }

    /**
     * Bind event listeners
     */
    bindEvents() {
        // Login form
        const loginForm = document.getElementById('login-form');
        if (loginForm) {
            loginForm.addEventListener('submit', (e) => this.handleLogin(e));
        }

        // Logout button
        const logoutBtn = document.getElementById('logout-btn');
        if (logoutBtn) {
            logoutBtn.addEventListener('click', () => this.handleLogout());
        }

        // Refresh button
        const refreshBtn = document.getElementById('refresh-btn');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', () => window.Dashboard.refreshData());
        }

        // Create Form button
        const createFormBtn = document.getElementById('create-form-btn');
        if (createFormBtn) {
            createFormBtn.addEventListener('click', () => window.FormsManager.showCreateForm());
        }

        // Search and filters
        this.setupSearchAndFilters();

        // Pagination
        this.setupPagination();
    }

    /**
     * Setup search and filter event listeners
     */
    setupSearchAndFilters() {
        // Search input
        const searchInput = document.getElementById('search-input');
        if (searchInput) {
            const debouncedSearch = window.UI.debounce((value) => {
                window.Dashboard.handleSearch(value);
            }, 500);
            
            searchInput.addEventListener('input', (e) => {
                debouncedSearch(e.target.value);
            });
        }

        // Status filter
        const filterStatus = document.getElementById('filter-status');
        if (filterStatus) {
            filterStatus.addEventListener('change', (e) => {
                window.Dashboard.handleFilter('status', e.target.value);
            });
        }

        // Type filter
        const filterType = document.getElementById('filter-type');
        if (filterType) {
            filterType.addEventListener('change', (e) => {
                window.Dashboard.handleFilter('type', e.target.value);
            });
        }
    }

    /**
     * Setup pagination event listeners
     */
    setupPagination() {
        const prevBtn = document.getElementById('prev-page');
        const nextBtn = document.getElementById('next-page');
        
        if (prevBtn) {
            prevBtn.addEventListener('click', () => window.Dashboard.previousPage());
        }
        
        if (nextBtn) {
            nextBtn.addEventListener('click', () => window.Dashboard.nextPage());
        }
    }

    /**
     * Handle login form submission
     */
    async handleLogin(event) {
        event.preventDefault();
        
        const form = event.target;
        const submitBtn = form.querySelector('button[type="submit"]');
        const secretInput = form.querySelector('#secret');
        const userIdInput = form.querySelector('#userId');
        
        const secret = secretInput.value.trim();
        const userId = userIdInput.value.trim();
        
        if (!secret || !userId) {
            window.UI.showError('Please fill in all fields');
            return;
        }
        
        try {
            window.UI.hideError();
            window.UI.setButtonLoading(submitBtn, true);
            
            // Attempt login
            const result = await window.AuthManager.login(secret, userId);
            
            if (result.success) {
                // Set API token
                window.APIClient.setToken(result.token);
                
                // Test authentication
                await window.APIClient.testAuth();
                
                window.UI.showToast('Successfully authenticated!', 'success');
                await window.Dashboard.showDashboard();
            } else {
                window.UI.showError(result.error || 'Authentication failed');
            }
            
        } catch (error) {
            console.error('Login error:', error);
            window.UI.showError('Authentication failed: ' + error.message);
        } finally {
            window.UI.setButtonLoading(submitBtn, false);
        }
    }

    /**
     * Handle logout
     */
    handleLogout() {
        window.AuthManager.logout();
        window.APIClient.clearAuth();
        window.Dashboard.clearAutoRefresh();
        window.UI.closeAllModals();
        window.UI.showToast('Logged out successfully', 'info');
        window.Dashboard.showLogin();
    }
}

// Initialize the application
const adminPanel = new AdminPanel();

// Make it globally available for event handlers
window.AdminPanel = adminPanel;
