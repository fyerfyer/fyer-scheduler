<template>
  <div class="pagination-container">
    <div class="pagination-info">
      Showing {{ startItem }}-{{ endItem }} of {{ total }} items
    </div>
    <div class="pagination-controls">
      <button
        class="pagination-btn"
        :disabled="currentPage === 1"
        @click="changePage(1)"
      >
        &laquo;
      </button>
      <button
        class="pagination-btn"
        :disabled="currentPage === 1"
        @click="changePage(currentPage - 1)"
      >
        &lsaquo;
      </button>

      <template v-for="page in visiblePages" :key="page">
        <button
          v-if="page !== '...'"
          class="pagination-btn"
          :class="{ active: page === currentPage }"
          @click="changePage(page)"
        >
          {{ page }}
        </button>
        <span v-else class="pagination-ellipsis">...</span>
      </template>

      <button
        class="pagination-btn"
        :disabled="currentPage === totalPages"
        @click="changePage(currentPage + 1)"
      >
        &rsaquo;
      </button>
      <button
        class="pagination-btn"
        :disabled="currentPage === totalPages"
        @click="changePage(totalPages)"
      >
        &raquo;
      </button>
    </div>
    <div class="pagination-size">
      <select :value="pageSize" @change="changePageSize">
        <option v-for="size in pageSizeOptions" :key="size" :value="size">
          {{ size }} per page
        </option>
      </select>
    </div>
  </div>
</template>

<script>
export default {
  name: 'Pagination',
  props: {
    currentPage: {
      type: Number,
      required: true
    },
    pageSize: {
      type: Number,
      required: true
    },
    total: {
      type: Number,
      required: true
    },
    pageSizeOptions: {
      type: Array,
      default: () => [10, 20, 50, 100]
    }
  },
  computed: {
    totalPages() {
      return Math.max(1, Math.ceil(this.total / this.pageSize));
    },
    startItem() {
      return this.total === 0 ? 0 : (this.currentPage - 1) * this.pageSize + 1;
    },
    endItem() {
      return Math.min(this.currentPage * this.pageSize, this.total);
    },
    visiblePages() {
      const pages = [];
      const maxVisible = 5;

      if (this.totalPages <= maxVisible) {
        // Show all pages if there are few
        for (let i = 1; i <= this.totalPages; i++) {
          pages.push(i);
        }
      } else {
        // Always show first page
        pages.push(1);

        // Calculate center pages
        let startPage = Math.max(2, this.currentPage - 1);
        let endPage = Math.min(this.totalPages - 1, this.currentPage + 1);

        // Add ellipsis before center if needed
        if (startPage > 2) {
          pages.push('...');
        }

        // Add center pages
        for (let i = startPage; i <= endPage; i++) {
          pages.push(i);
        }

        // Add ellipsis after center if needed
        if (endPage < this.totalPages - 1) {
          pages.push('...');
        }

        // Always show last page
        pages.push(this.totalPages);
      }

      return pages;
    }
  },
  methods: {
    changePage(page) {
      if (page !== this.currentPage) {
        this.$emit('update:currentPage', page);
      }
    },
    changePageSize(event) {
      const newSize = parseInt(event.target.value);
      this.$emit('update:pageSize', newSize);
    }
  }
};
</script>

<style scoped>
.pagination-container {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 1rem 0;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.pagination-info {
  font-size: 0.85rem;
  color: #606266;
}

.pagination-controls {
  display: flex;
  align-items: center;
}

.pagination-btn {
  min-width: 32px;
  height: 32px;
  margin: 0 4px;
  padding: 0 4px;
  background: white;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.pagination-btn:hover:not(:disabled) {
  color: #1890ff;
  border-color: #1890ff;
}

.pagination-btn:disabled {
  cursor: not-allowed;
  color: #d9d9d9;
}

.pagination-btn.active {
  background: #1890ff;
  color: white;
  border-color: #1890ff;
}

.pagination-ellipsis {
  display: inline-block;
  width: 24px;
  text-align: center;
}

.pagination-size select {
  padding: 4px 8px;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  background: white;
  cursor: pointer;
}
</style>