<script>
	import {
		wsUrl,
	} from '@/common/request/api.js'
	export default {
		// 全局变量 全端支持
		globalData: {
			StatusBar: 0,
			CustomBar: 0,
			Custom: 0,
			userInfo: {},
			avatar: "https://jiang-xia.top/x-blog/api/v1/static/uploads/2023-01-01/2kf3d768tj33vsgs6exbu8-默认头像.jpeg",
			socketTask: null,
		},
		data() {
			return {
				timer: null
			}
		},
		/* 应用生命周期 */
		onLaunch: function() {
			// console.warn('当前组件仅支持 uni_modules 目录结构 ，请升级 HBuilderX 到 3.1.0 版本以上！')
			console.log('App Launch')
			this.initPush()
			const that = this
			uni.getSystemInfo({
				success: function(e) {
					// #ifndef MP
					that.globalData.StatusBar = e.statusBarHeight;
					if (e.platform == 'android') {
						that.globalData.CustomBar = e.statusBarHeight + 50;
					} else {
						that.globalData.CustomBar = e.statusBarHeight + 45;
					};
					// #endif
					// #ifdef MP-WEIXIN
					that.globalData.StatusBar = e.statusBarHeight;
					let custom = wx.getMenuButtonBoundingClientRect();
					that.globalData.Custom = custom;
					that.globalData.CustomBar = custom.bottom + custom.top - e.statusBarHeight;
					// #endif		
					// #ifdef MP-ALIPAY
					that.globalData.StatusBar = e.statusBarHeight;
					that.globalData.CustomBar = e.statusBarHeight + e.titleBarHeight;
					// #endif
					// that.initOpt();
					// console.log(that.globalData)
				}
			})
			const userInfo = uni.getStorageSync("zoneUserInfo")
			if (userInfo) {
				this.globalData.userInfo = userInfo
				this.initChat(userInfo.userId)
			}

			/* 全局用户信息处理 */
			if (uni.getStorageSync("zoneToken") && !userInfo) {
				this.$api.get('/base/users/info').then(res => {
					uni.setStorageSync('zoneUserInfo', res.data)
					this.globalData.userInfo = res.data
					this.initChat(res.data.userId)
				}).catch((err) => {
					uni.setStorageSync("zoneToken", "")
					uni.setStorageSync('zoneUserInfo', "")
				})
			}
			// #ifdef MP-WEIXIN
			uni.showShareMenu({
				// 小程序分享
				withShareTicket: true,
			})
			// #endif
		},
		onShow: function() {
			console.log('App Show');
			// #ifdef MP-WEIXIN
			this.mpVersionUpdate()
			// #endif

		},
		onReady: function() {
			console.log('App onReady');
			this.initOpt();
		},
		onHide: function() {
			console.log('App Hide')
		},
		methods: {
			// 初始化操作
			initOpt() {
				// console.log(window)
				window.addEventListener('resize', function() {
					if (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName ===
						'TEXTAREA') { //滚动到当前元素的方法
						window.setTimeout(function() {
							if ('scrollIntoView' in document.activeElement) {
								document.activeElement.scrollIntoView(false)
							} else {
								document.activeElement.scrollIntoViewIfNeeded(false)
							}
						}, 0)
					}
				})
			},
			//版本更新
			mpVersionUpdate() {
				const updateManager = uni.getUpdateManager();
				updateManager.onCheckForUpdate(function(res) {
					console.log("hasUpdate", res.hasUpdate)
					// 请求完新版本信息的回调
					if (res.hasUpdate) {
						uni.showLoading({
							title: '正在下载'
						})
						updateManager.onUpdateReady(function(res) {
							uni.hideLoading()
							uni.showModal({
								title: '更新提示',
								content: '新版本已经准备好，是否重启应用？',
								success(res) {
									if (res.confirm) {
										// 新的版本已经下载好，调用 applyUpdate 应用新版本并重启
										updateManager.applyUpdate();
									} else {
										uni.showModal({
											content: '本次版本更新涉及到新功能的添加，旧版本将无法正常使用',
											showCancel: false,
											confirmText: '确认更新'
										}).then(res => {
											updateManager.applyUpdate();
										});
									}
								}
							});
						})
						updateManager.onUpdateFailed(function(res) {
							uni.hideLoading()
							uni.showToast({
								title: '新的版本下载失败'
							})
						});

					}
				})
			},
			// 初始化聊天
			initChat(userId) {
				console.log({
					userId
				})
				const url = wsUrl + '/mobile/chat?userId=' + userId
				const token = uni.getStorageSync("zoneToken")
				this.globalData.socketTask = uni.connectSocket({
					url,
					method: 'GET',
					complete: () => {
						console.log('WebSocket已连接！');
					}
				});
				// 监听服务端发送消息
				this.globalData.socketTask.onMessage((res) => {
					// if (res.data) {
					// 	const revObj = JSON.parse(res.data)
					// 	console.log('服务端消息：', revObj);
					// 	}
				});
				this.globalData.socketTask.onOpen((res) => {
					console.log('WebSocket连接已打开！');
					this.heartbeat()
				});
				this.globalData.socketTask.onError((res) => {
					console.log('WebSocket连接打开失败，请检查！');
				});
			},
			heartbeat(userId) {
				this.timer = setInterval(() => {
					this.globalData.socketTask.send({
						data: JSON.stringify({
							cmd: "heartbeat",
							content: "heartbeat",
							senderId: this.globalData.userInfo.userId,
						}),
						success: () => {
							// console.log('发送成功：');
						},
						fail: (error) => {
							console.log('发送失败:', error);
						}
					});
				}, 10000)
			},
			// 初始化推送功能
			initPush() {
				Notification.requestPermission().then(permission => {
				  console.log(permission)
				})
				uni.getPushClientId({
					success: (res) => {
						console.log(res);
					},
					fail(err) {
						console.log(err)
					}
				})
				
				uni.onPushMessage((res) => {
					console.log('onPushMessage',res)
					//获取到通知消息有分为点击与监听
					if (res.type == 'click') {
						clearTimeout(timer);

						timer = setTimeout(() => {
							// 处理跳转的业务可以写这边
						}, 1500);
					} else {
						//aps为空说明是在线，aps有值的时候是离线，避免离线点击后再监听会出现两次通知栏
						if (res.data.aps) {
							return;
						}
						let platform = uni.getSystemInfoSync().platform;
						let msg = res.data;
						if (platform == 'ios') { //由于IOS 必须要创建本地消息 所以做这个判断
							if (msg.payload && msg.payload != null) {
								plus.push.createMessage(msg.content, msg.payload) //创建本地消息
							}
						}
						if (platform == 'android') {
							let options = {
								cover: false,
								sound: "system",
								title: msg.title
							}
							plus.push.createMessage(msg.content, msg.payload, options);
						}


					}
				})

			}
		}
	}
</script>
<style lang="scss">
	@import "@/static/iconfont.css";
	@import "common/css/base.scss";
	@import "common/css/animate.min.css";
	/*每个页面公共css 引入辅助样式*/
	@import '@/uni_modules/uni-scss/index.scss';
	// 编辑器样式
	@import 'common/css/editor-v3.style.css';

	/* #ifndef APP-NVUE */
	// 设置整个项目的背景色
	page {
		// background-color: #f5f5f5;
		user-select: auto;
	}

	/* #endif */

	// 自定义导航栏公共样式
	.uni-navbar {
		.nav-title {
			font-size: 32rpx;
			text-align: center;
			margin: auto;
		}

		.uni-nav-bar-text {
			font-size: 32rpx !important;
		}

		.uni-navbar-btn-text {
			line-height: 44rpx !important;

			text {
				font-size: 32rpx !important;
			}
		}
	}
</style>