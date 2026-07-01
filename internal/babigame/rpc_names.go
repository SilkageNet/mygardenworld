package babigame

import "sort"

const (
	RPCPlantRqstZhtc                            RPCName = "PlantRqst.zhtc"
	RPCReapPopupShjm                            RPCName = "ReapPopup.shjm"
	RPCActBuy                                   RPCName = "act.buy"
	RPCActGetOneOrderAward                      RPCName = "act.getOneOrderAward"
	RPCActGetOrderAward                         RPCName = "act.getOrderAward"
	RPCActGetRankAward                          RPCName = "act.getRankAward"
	RPCActGetStat                               RPCName = "act.getStat"
	RPCActGiftBuy                               RPCName = "act.giftBuy"
	RPCActRecv                                  RPCName = "act.recv"
	RPCActRecvBoxes                             RPCName = "act.recvBoxes"
	RPCActRecvTLAward                           RPCName = "act.recvTLAward"
	RPCActRefreshDailyGift                      RPCName = "act.refreshDailyGift"
	RPCActRefreshTask                           RPCName = "act.refreshTask"
	RPCActSyncBatchInfo                         RPCName = "act.syncBatchInfo"
	RPCActCallBackActCallBackBind               RPCName = "actCallBack.actCallBackBind"
	RPCActCallBackActCallBackEnter              RPCName = "actCallBack.actCallBackEnter"
	RPCActCallBackActCallBackRecv               RPCName = "actCallBack.actCallBackRecv"
	RPCActCardCollectCheckLuckyCardSend         RPCName = "actCardCollect.checkLuckyCardSend"
	RPCActCardCollectDeckShopExchange           RPCName = "actCardCollect.deckShopExchange"
	RPCActCardCollectNextRound                  RPCName = "actCardCollect.nextRound"
	RPCActCardCollectOpenCardPack               RPCName = "actCardCollect.openCardPack"
	RPCActCardCollectRecvBoxReward              RPCName = "actCardCollect.recvBoxReward"
	RPCActCardCollectRecvCardAlbumReward        RPCName = "actCardCollect.recvCardAlbumReward"
	RPCActCardCollectRecvCollectReward          RPCName = "actCardCollect.recvCollectReward"
	RPCActCardCollectRecvTaskReward             RPCName = "actCardCollect.recvTaskReward"
	RPCActCardCollectRefreshTaskData            RPCName = "actCardCollect.refreshTaskData"
	RPCActCardCollectUseSelectedCard            RPCName = "actCardCollect.useSelectedCard"
	RPCActCyclicNoteDirectRecvTaskRwd           RPCName = "actCyclicNote.directRecvTaskRwd"
	RPCActCyclicNoteGiftBuy                     RPCName = "actCyclicNote.giftBuy"
	RPCActCyclicNoteReRandomTask                RPCName = "actCyclicNote.reRandomTask"
	RPCActCyclicNoteRecv                        RPCName = "actCyclicNote.recv"
	RPCActCyclicNoteRecvTaskRwd                 RPCName = "actCyclicNote.recvTaskRwd"
	RPCActCyclicNoteResetGiftCd                 RPCName = "actCyclicNote.resetGiftCd"
	RPCActCyclicNoteUnlockTaskSlot              RPCName = "actCyclicNote.unlockTaskSlot"
	RPCActCyclicStoryGiftBuy                    RPCName = "actCyclicStory.giftBuy"
	RPCActCyclicStoryReRandomOrder              RPCName = "actCyclicStory.reRandomOrder"
	RPCActCyclicStoryRecv                       RPCName = "actCyclicStory.recv"
	RPCActCyclicStoryRecvOrderRwd               RPCName = "actCyclicStory.recvOrderRwd"
	RPCActCyclicStoryRemoveOrderCd              RPCName = "actCyclicStory.removeOrderCd"
	RPCActCyclicStoryResetGiftCd                RPCName = "actCyclicStory.resetGiftCd"
	RPCActCyclicVaseGiftBuy                     RPCName = "actCyclicVase.giftBuy"
	RPCActCyclicVaseRecv                        RPCName = "actCyclicVase.recv"
	RPCActCyclicVaseResetGiftCd                 RPCName = "actCyclicVase.resetGiftCd"
	RPCActDessertEnter                          RPCName = "actDessert.enter"
	RPCActDessertGameOver                       RPCName = "actDessert.gameOver"
	RPCActDessertGameStart                      RPCName = "actDessert.gameStart"
	RPCActDessertGameSync                       RPCName = "actDessert.gameSync"
	RPCActDessertGiftBuy                        RPCName = "actDessert.giftBuy"
	RPCActDessertOpenBox                        RPCName = "actDessert.openBox"
	RPCActDrawDraw                              RPCName = "actDraw.draw"
	RPCActDrawChristmasDraw                     RPCName = "actDrawChristmas.draw"
	RPCActDrawChristmasEnter                    RPCName = "actDrawChristmas.enter"
	RPCActDrawChristmasGiftBuy                  RPCName = "actDrawChristmas.giftBuy"
	RPCActDrawDragonDraw                        RPCName = "actDrawDragon.draw"
	RPCActDrawDragonGiftBuy                     RPCName = "actDrawDragon.giftBuy"
	RPCActDrawDragonRecv                        RPCName = "actDrawDragon.recv"
	RPCActDrawGiftGiftBuy                       RPCName = "actDrawGift.giftBuy"
	RPCActDrawSprSkinDraw                       RPCName = "actDrawSprSkin.draw"
	RPCActDrawSprSkinEnter                      RPCName = "actDrawSprSkin.enter"
	RPCActDrawSprSkinGiftBuy                    RPCName = "actDrawSprSkin.giftBuy"
	RPCActDrawZbDraw                            RPCName = "actDrawZb.draw"
	RPCActDrawZbEnter                           RPCName = "actDrawZb.enter"
	RPCActDrawZbGiftBuy                         RPCName = "actDrawZb.giftBuy"
	RPCActElimEnter                             RPCName = "actElim.enter"
	RPCActElimGiftBuy                           RPCName = "actElim.giftBuy"
	RPCActElimMove                              RPCName = "actElim.move"
	RPCActElimOpenBox                           RPCName = "actElim.openBox"
	RPCActElimRefreshMap                        RPCName = "actElim.refreshMap"
	RPCActElimUseItem1                          RPCName = "actElim.useItem1"
	RPCActElimUseItem2                          RPCName = "actElim.useItem2"
	RPCActFlowerBattleChooseFlowerArt           RPCName = "actFlowerBattle.chooseFlowerArt"
	RPCActFlowerBattleEnter                     RPCName = "actFlowerBattle.enter"
	RPCActFlowerBattleGetGiftBuyRecords         RPCName = "actFlowerBattle.getGiftBuyRecords"
	RPCActFlowerBattleGiftBuy                   RPCName = "actFlowerBattle.giftBuy"
	RPCActFlowerBattleLike                      RPCName = "actFlowerBattle.like"
	RPCActFlowerBattleRecvBoxesPrize            RPCName = "actFlowerBattle.recvBoxesPrize"
	RPCActFlowerBattleSetIsAnonymous            RPCName = "actFlowerBattle.setIsAnonymous"
	RPCActFmlRedEnvelopeEnter                   RPCName = "actFmlRedEnvelope.enter"
	RPCActFmlRedEnvelopeGetDetail               RPCName = "actFmlRedEnvelope.getDetail"
	RPCActFmlRedEnvelopeGetRecord               RPCName = "actFmlRedEnvelope.getRecord"
	RPCActFmlRedEnvelopeList                    RPCName = "actFmlRedEnvelope.list"
	RPCActFmlRedEnvelopePick                    RPCName = "actFmlRedEnvelope.pick"
	RPCActFmlRedEnvelopeSend                    RPCName = "actFmlRedEnvelope.send"
	RPCActGame2048Enter                         RPCName = "actGame2048.enter"
	RPCActGame2048GiftBuy                       RPCName = "actGame2048.giftBuy"
	RPCActGame2048Move                          RPCName = "actGame2048.move"
	RPCActGame2048OpenBox                       RPCName = "actGame2048.openBox"
	RPCActGame2048Restart                       RPCName = "actGame2048.restart"
	RPCActGame2048UseChange                     RPCName = "actGame2048.useChange"
	RPCActGame2048UseEliminate                  RPCName = "actGame2048.useEliminate"
	RPCActHoneyGiftBuy                          RPCName = "actHoney.giftBuy"
	RPCActHoneyRecv                             RPCName = "actHoney.recv"
	RPCActHoneyResetGiftCd                      RPCName = "actHoney.resetGiftCd"
	RPCActIPDmdGiftGiftBuy                      RPCName = "actIPDmdGift.giftBuy"
	RPCActIPFlowerGuardOpenBox                  RPCName = "actIPFlowerGuard.openBox"
	RPCActMerge2Enter                           RPCName = "actMerge2.enter"
	RPCActMerge2Move                            RPCName = "actMerge2.move"
	RPCActMerge2OpenBox                         RPCName = "actMerge2.openBox"
	RPCActMerge2PutInWarehouse                  RPCName = "actMerge2.putInWarehouse"
	RPCActMerge2PutOutTemp                      RPCName = "actMerge2.putOutTemp"
	RPCActMerge2PutOutWarehouse                 RPCName = "actMerge2.putOutWarehouse"
	RPCActMerge2RecvOrder                       RPCName = "actMerge2.recvOrder"
	RPCActMerge2RecvProgress                    RPCName = "actMerge2.recvProgress"
	RPCActMerge2RefreshOrder                    RPCName = "actMerge2.refreshOrder"
	RPCActMerge2SaveGuide                       RPCName = "actMerge2.saveGuide"
	RPCActMerge2SellItem                        RPCName = "actMerge2.sellItem"
	RPCActMerge2SplitItem                       RPCName = "actMerge2.splitItem"
	RPCActMerge2SwitchMode                      RPCName = "actMerge2.switchMode"
	RPCActMerge2UnlockWarehouse                 RPCName = "actMerge2.unlockWarehouse"
	RPCActMerge2UseItem                         RPCName = "actMerge2.useItem"
	RPCActOfficialsBuyItem                      RPCName = "actOfficials.buyItem"
	RPCActOfficialsEnter                        RPCName = "actOfficials.enter"
	RPCActOfficialsRecvGrpReachPrize            RPCName = "actOfficials.recvGrpReachPrize"
	RPCActOfficialsUseItem                      RPCName = "actOfficials.useItem"
	RPCActPaperEnter                            RPCName = "actPaper.enter"
	RPCActPaperRecv                             RPCName = "actPaper.recv"
	RPCActPaperRecvGamePrize                    RPCName = "actPaper.recvGamePrize"
	RPCActPaperRecvTaskPrize                    RPCName = "actPaper.recvTaskPrize"
	RPCActRchgRwdEnter                          RPCName = "actRchgRwd.enter"
	RPCActRchgRwdRecv                           RPCName = "actRchgRwd.recv"
	RPCActRchgWheelEnter                        RPCName = "actRchgWheel.enter"
	RPCActRchgWheelGetMyLog                     RPCName = "actRchgWheel.getMyLog"
	RPCActRchgWheelStartWheel                   RPCName = "actRchgWheel.startWheel"
	RPCActSpoolEnter                            RPCName = "actSpool.enter"
	RPCActSpoolGameOver                         RPCName = "actSpool.gameOver"
	RPCActSpoolGameStart                        RPCName = "actSpool.gameStart"
	RPCActSpoolGameSync                         RPCName = "actSpool.gameSync"
	RPCActSpoolGiftBuy                          RPCName = "actSpool.giftBuy"
	RPCActSpoolOpenBox                          RPCName = "actSpool.openBox"
	RPCActSpoolRise                             RPCName = "actSpool.rise"
	RPCActSpoolSetGuideStatus                   RPCName = "actSpool.setGuideStatus"
	RPCActSpringTotRchgRecvTLAward              RPCName = "actSpringTotRchg.recvTLAward"
	RPCActVipTimeShopGiftBuy                    RPCName = "actVipTimeShop.giftBuy"
	RPCActZFBForestBrowseWeb                    RPCName = "actZFBForest.browseWeb"
	RPCActZFBForestBrowseWeb2                   RPCName = "actZFBForest.browseWeb2"
	RPCBagCombine                               RPCName = "bag.combine"
	RPCBagSell                                  RPCName = "bag.sell"
	RPCBagUse                                   RPCName = "bag.use"
	RPCBattlePassBuyLvl                         RPCName = "battlePass.buyLvl"
	RPCBattlePassRecv                           RPCName = "battlePass.recv"
	RPCBattlePassRecvAll                        RPCName = "battlePass.recvAll"
	RPCBattlePassTaskDone                       RPCName = "battlePass.taskDone"
	RPCBenefitBoxDraw                           RPCName = "benefitBox.draw"
	RPCBestieApply                              RPCName = "bestie.apply"
	RPCBestieCancelDissolve                     RPCName = "bestie.cancelDissolve"
	RPCBestieCheckApply                         RPCName = "bestie.checkApply"
	RPCBestieDissolve                           RPCName = "bestie.dissolve"
	RPCBestieEnter                              RPCName = "bestie.enter"
	RPCBestieGetFrdBestieCntMap                 RPCName = "bestie.getFrdBestieCntMap"
	RPCBestieHandleApply                        RPCName = "bestie.handleApply"
	RPCBestieImmediateDissolve                  RPCName = "bestie.immediateDissolve"
	RPCBestieSetSceneSkin                       RPCName = "bestie.setSceneSkin"
	RPCBestieUnlockSlot                         RPCName = "bestie.unlockSlot"
	RPCBoostRecvRwd                             RPCName = "boost.recvRwd"
	RPCBoostRefresh                             RPCName = "boost.refresh"
	RPCBubbleActiveBubble                       RPCName = "bubble.activeBubble"
	RPCBubbleChgBubble                          RPCName = "bubble.chgBubble"
	RPCCallFriendEnter                          RPCName = "callFriend.enter"
	RPCCallFriendRecv                           RPCName = "callFriend.recv"
	RPCCallFriendUseCode                        RPCName = "callFriend.useCode"
	RPCCelebrityGetAllTypes                     RPCName = "celebrity.getAllTypes"
	RPCCelebrityGetAllTypesInfo                 RPCName = "celebrity.getAllTypesInfo"
	RPCCelebrityGetInfoByType                   RPCName = "celebrity.getInfoByType"
	RPCCelebrityLikeCelebrity                   RPCName = "celebrity.likeCelebrity"
	RPCChannelRwdRecvDailyDesktopRwd            RPCName = "channelRwd.recvDailyDesktopRwd"
	RPCChannelRwdRecvFstDesktopRwd              RPCName = "channelRwd.recvFstDesktopRwd"
	RPCChannelRwdRecvFstSidebarRwd              RPCName = "channelRwd.recvFstSidebarRwd"
	RPCChannelRwdRecvLoginRwd                   RPCName = "channelRwd.recvLoginRwd"
	RPCCheaterDoCheat                           RPCName = "cheater.doCheat"
	RPCCollectRwdRecv                           RPCName = "collectRwd.recv"
	RPCCollectRwdRecvArtCreateRwd               RPCName = "collectRwd.recvArtCreateRwd"
	RPCCollectRwdRecvArtCreateByVase            RPCName = "collectRwd.recvArtCreateRwdByVase"
	RPCCultivateChooseSkill                     RPCName = "cultivate.chooseSkill"
	RPCCultivateClearCulCD                      RPCName = "cultivate.clearCulCd"
	RPCCultivateCultivate                       RPCName = "cultivate.cultivate"
	RPCCultivateRandomSkill                     RPCName = "cultivate.randomSkill"
	RPCCultivateRecv                            RPCName = "cultivate.recv"
	RPCCultivateReduceByHelp                    RPCName = "cultivate.reduceByHelp"
	RPCCultivateReduceByItem                    RPCName = "cultivate.reduceByItem"
	RPCCultivateUnlockSlot                      RPCName = "cultivate.unlockSlot"
	RPCCultivateUpgrade                         RPCName = "cultivate.upgrade"
	RPCCustomerOrderRqstDkgkck                  RPCName = "customerOrderRqst.dkgkck"
	RPCDecorateBuild                            RPCName = "decorate.build"
	RPCDecorateBuildSuccess                     RPCName = "decorate.buildSuccess"
	RPCDecorateClearBuildCd                     RPCName = "decorate.clearBuildCd"
	RPCDecorateEquip                            RPCName = "decorate.equip"
	RPCDecorateRecv                             RPCName = "decorate.recv"
	RPCDecorateUpdateReadLvlList                RPCName = "decorate.updateReadLvlList"
	RPCDrawDraw                                 RPCName = "draw.draw"
	RPCDrawTestDrawVirtual                      RPCName = "draw.testDrawVirtual"
	RPCFashionEquip                             RPCName = "fashion.equip"
	RPCFashionRead                              RPCName = "fashion.read"
	RPCFlowerArtMakeFlowerArt                   RPCName = "flowerArt.makeFlowerArt"
	RPCFlowerElvesCheckConvert                  RPCName = "flowerElves.checkConvert"
	RPCFlowerElvesAidHelpFrd                    RPCName = "flowerElvesAid.helpFrd"
	RPCFlowerElvesAidRecvAidEff                 RPCName = "flowerElvesAid.recvAidEff"
	RPCFlowerElvesAidReqAid                     RPCName = "flowerElvesAid.reqAid"
	RPCFlowerElvesBookUpgrade                   RPCName = "flowerElvesBook.upgrade"
	RPCFlowerElvesBookDrawDraw                  RPCName = "flowerElvesBookDraw.draw"
	RPCFlowerElvesBookDrawRefresh               RPCName = "flowerElvesBookDraw.refresh"
	RPCFlowerElvesPlaceDispatch                 RPCName = "flowerElvesPlace.dispatch"
	RPCFlowerElvesPlaceRecv                     RPCName = "flowerElvesPlace.recv"
	RPCFlowerElvesPlaceRecvAllReward            RPCName = "flowerElvesPlace.recvAllReward"
	RPCFlowerElvesPlaceSpeedUp                  RPCName = "flowerElvesPlace.speedUp"
	RPCFlowerElvesPlaceUnlock                   RPCName = "flowerElvesPlace.unlock"
	RPCFlowerGiftRecvBox                        RPCName = "flowerGift.recvBox"
	RPCFlowerMarketBuyFlower                    RPCName = "flowerMarket.buyFlower"
	RPCFlowerMarketBuyPutCount                  RPCName = "flowerMarket.buyPutCount"
	RPCFlowerMarketCheckPassword                RPCName = "flowerMarket.checkPassword"
	RPCFlowerMarketEnter                        RPCName = "flowerMarket.enter"
	RPCFlowerMarketGetFriend                    RPCName = "flowerMarket.getFriend"
	RPCFlowerMarketGetFriendList                RPCName = "flowerMarket.getFriendList"
	RPCFlowerMarketGetTradeRecords              RPCName = "flowerMarket.getTradeRecords"
	RPCFlowerMarketHarvestIncome                RPCName = "flowerMarket.harvestIncome"
	RPCFlowerMarketPutFlower                    RPCName = "flowerMarket.putFlower"
	RPCFlowerMarketPutFlowerBatch               RPCName = "flowerMarket.putFlowerBatch"
	RPCFlowerMarketTakeDownFlower               RPCName = "flowerMarket.takeDownFlower"
	RPCFlowerMarketUnlockShelf                  RPCName = "flowerMarket.unlockShelf"
	RPCFlowerOrderRqstShowR                     RPCName = "flowerOrderRqst.showR"
	RPCFlowerRackCancelSell                     RPCName = "flowerRack.cancelSell"
	RPCFlowerRackRecvOneKey                     RPCName = "flowerRack.recvOneKey"
	RPCFlowerRackRecvSellMoney                  RPCName = "flowerRack.recvSellMoney"
	RPCFlowerRackSell                           RPCName = "flowerRack.sell"
	RPCFlowerRackUnlockStand                    RPCName = "flowerRack.unlockStand"
	RPCFmlAutoJoin                              RPCName = "fml.autoJoin"
	RPCFmlBld                                   RPCName = "fml.bld"
	RPCFmlBuyRaceBoat                           RPCName = "fml.buyRaceBoat"
	RPCFmlChgPos                                RPCName = "fml.chgPos"
	RPCFmlChgTitle                              RPCName = "fml.chgTitle"
	RPCFmlClearQuitTime                         RPCName = "fml.clearQuitTime"
	RPCFmlCreate                                RPCName = "fml.create"
	RPCFmlDissolve                              RPCName = "fml.dissolve"
	RPCFmlEnter                                 RPCName = "fml.enter"
	RPCFmlEquipRaceBoat                         RPCName = "fml.equipRaceBoat"
	RPCFmlGetHonor                              RPCName = "fml.getHonor"
	RPCFmlGetLog                                RPCName = "fml.getLog"
	RPCFmlGetMedalRwd                           RPCName = "fml.getMedalRwd"
	RPCFmlGetRecFmlList                         RPCName = "fml.getRecFmlList"
	RPCFmlGetTitleLogList                       RPCName = "fml.getTitleLogList"
	RPCFmlHandleApply                           RPCName = "fml.handleApply"
	RPCFmlHandleApplyAll                        RPCName = "fml.handleApplyAll"
	RPCFmlHandleInv                             RPCName = "fml.handleInv"
	RPCFmlInv                                   RPCName = "fml.inv"
	RPCFmlJoin                                  RPCName = "fml.join"
	RPCFmlKick                                  RPCName = "fml.kick"
	RPCFmlOpenFmlRaceBox                        RPCName = "fml.openFmlRaceBox"
	RPCFmlQuit                                  RPCName = "fml.quit"
	RPCFmlRecvBox                               RPCName = "fml.recvBox"
	RPCFmlRefreshRaceBoat                       RPCName = "fml.refreshRaceBoat"
	RPCFmlRefreshTitle                          RPCName = "fml.refreshTitle"
	RPCFmlSearch                                RPCName = "fml.search"
	RPCFmlSetting                               RPCName = "fml.setting"
	RPCFmlUnbindUnionGroup                      RPCName = "fml.unbindUnionGroup"
	RPCFmlUnloadRaceBoat                        RPCName = "fml.unloadRaceBoat"
	RPCFmlUpgradeFml                            RPCName = "fml.upgradeFml"
	RPCFmlUpgradeRaceBoat                       RPCName = "fml.upgradeRaceBoat"
	RPCFmlFlowerShareAddTakeCnt                 RPCName = "fmlFlowerShare.addTakeCnt"
	RPCFmlFlowerShareGetFmlOtherShareList       RPCName = "fmlFlowerShare.getFmlOtherShareList"
	RPCFmlFlowerShareGetShareLogList            RPCName = "fmlFlowerShare.getShareLogList"
	RPCFmlFlowerShareRecvRwd                    RPCName = "fmlFlowerShare.recvRwd"
	RPCFmlFlowerShareRefresh                    RPCName = "fmlFlowerShare.refresh"
	RPCFmlFlowerShareShare                      RPCName = "fmlFlowerShare.share"
	RPCFmlFlowerShareTake                       RPCName = "fmlFlowerShare.take"
	RPCFmlFlowerShareUnlock                     RPCName = "fmlFlowerShare.unlock"
	RPCFmlFlowerShowCancelLikeOther             RPCName = "fmlFlowerShow.cancelLikeOther"
	RPCFmlFlowerShowGetLikeOtherRecord          RPCName = "fmlFlowerShow.getLikeOtherRecord"
	RPCFmlFlowerShowGetLikeOtherRecord5Limit    RPCName = "fmlFlowerShow.getLikeOtherRecord5Limit"
	RPCFmlFlowerShowGetShowInfo                 RPCName = "fmlFlowerShow.getShowInfo"
	RPCFmlFlowerShowLikeOther                   RPCName = "fmlFlowerShow.likeOther"
	RPCFmlFlowerShowSaveShow                    RPCName = "fmlFlowerShow.saveShow"
	RPCFmlFlowerShowSetVisitType                RPCName = "fmlFlowerShow.setVisitType"
	RPCFmlFlowerShowSwitchMap                   RPCName = "fmlFlowerShow.switchMap"
	RPCFmlFlowerShowUnlockSlot                  RPCName = "fmlFlowerShow.unlockSlot"
	RPCFmlForestApplyPlant                      RPCName = "fmlForest.applyPlant"
	RPCFmlForestCollectEnergy                   RPCName = "fmlForest.collectEnergy"
	RPCFmlForestEnter                           RPCName = "fmlForest.enter"
	RPCFmlForestGetCertDetail                   RPCName = "fmlForest.getCertDetail"
	RPCFmlForestGetCertDetailByFid              RPCName = "fmlForest.getCertDetailByFid"
	RPCFmlForestGetLogList                      RPCName = "fmlForest.getLogList"
	RPCFmlForestGetTreeList                     RPCName = "fmlForest.getTreeList"
	RPCFmlForestGetWeekCollect                  RPCName = "fmlForest.getWeekCollect"
	RPCFmlForestGetWeekStat                     RPCName = "fmlForest.getWeekStat"
	RPCFmlForestRefresh                         RPCName = "fmlForest.refresh"
	RPCFmlLandHarvest                           RPCName = "fmlLand.harvest"
	RPCFmlLandHarvestAll                        RPCName = "fmlLand.harvestAll"
	RPCFmlLandPlant                             RPCName = "fmlLand.plant"
	RPCFmlLandUnlock                            RPCName = "fmlLand.unlock"
	RPCFmlLandUpgrade                           RPCName = "fmlLand.upgrade"
	RPCFmlRaceBuyTaskNum                        RPCName = "fmlRace.buyTaskNum"
	RPCFmlRaceDelTask                           RPCName = "fmlRace.delTask"
	RPCFmlRaceEnter                             RPCName = "fmlRace.enter"
	RPCFmlRaceFinishTask                        RPCName = "fmlRace.finishTask"
	RPCFmlRaceGetFmlRaceEndDisplayData          RPCName = "fmlRace.getFmlRaceEndDisplayData"
	RPCFmlRaceGetFmlRaceHistRcdList             RPCName = "fmlRace.getFmlRaceHistRcdList"
	RPCFmlRaceGetFmlRaceTaskScore               RPCName = "fmlRace.getFmlRaceTaskScore"
	RPCFmlRaceGetFmlRaceUsrRankList             RPCName = "fmlRace.getFmlRaceUsrRankList"
	RPCFmlRaceGetGroupFmlRaceRcdList            RPCName = "fmlRace.getGroupFmlRaceRcdList"
	RPCFmlRaceGetNewMbScoreRank                 RPCName = "fmlRace.getNewMbScoreRank"
	RPCFmlRaceGetTaskList                       RPCName = "fmlRace.getTaskList"
	RPCFmlRaceGetTaskLogList                    RPCName = "fmlRace.getTaskLogList"
	RPCFmlRaceGiveUpTask                        RPCName = "fmlRace.giveUpTask"
	RPCFmlRaceRefreshFmlRaceBatch               RPCName = "fmlRace.refreshFmlRaceBatch"
	RPCFmlRaceRefreshFmlRaceBox                 RPCName = "fmlRace.refreshFmlRaceBox"
	RPCFmlRaceRefreshTask                       RPCName = "fmlRace.refreshTask"
	RPCFmlRaceTakeTask                          RPCName = "fmlRace.takeTask"
	RPCFmlRaceUpgradeTask                       RPCName = "fmlRace.upgradeTask"
	RPCFmlRaceRqstShowShip                      RPCName = "fmlRaceRqst.showShip"
	RPCFmlTaskEnterShowtcrw                     RPCName = "fmlTaskEnter.showtcrw"
	RPCFrdAddBlack                              RPCName = "frd.addBlack"
	RPCFrdApplyFrd                              RPCName = "frd.applyFrd"
	RPCFrdApplyFrdBatch                         RPCName = "frd.applyFrdBatch"
	RPCFrdDel                                   RPCName = "frd.del"
	RPCFrdDelBlack                              RPCName = "frd.delBlack"
	RPCFrdEnter                                 RPCName = "frd.enter"
	RPCFrdGetApplyList                          RPCName = "frd.getApplyList"
	RPCFrdGetBlackList                          RPCName = "frd.getBlackList"
	RPCFrdGetFriendList                         RPCName = "frd.getFriendList"
	RPCFrdHandleApply                           RPCName = "frd.handleApply"
	RPCFrdHandleApplyBatch                      RPCName = "frd.handleApplyBatch"
	RPCFrdLike                                  RPCName = "frd.like"
	RPCFrdRefreshRecList                        RPCName = "frd.refreshRecList"
	RPCFrdSetFrdRjt                             RPCName = "frd.setFrdRjt"
	RPCFrdExtBuyStealCnt                        RPCName = "frdExt.buyStealCnt"
	RPCFrdExtCancelFollow                       RPCName = "frdExt.cancelFollow"
	RPCFrdExtFollow                             RPCName = "frdExt.follow"
	RPCFrdExtGetFrdOtherInfoByUids              RPCName = "frdExt.getFrdOtherInfoByUids"
	RPCFrdExtSearchUser                         RPCName = "frdExt.searchUser"
	RPCFrdHomeGetFrdHomeInfo                    RPCName = "frdHome.getFrdHomeInfo"
	RPCFrdShareEnter                            RPCName = "frdShare.enter"
	RPCFrdShareRecvBoxRwd                       RPCName = "frdShare.recvBoxRwd"
	RPCFrdShareRecvSelfRwd                      RPCName = "frdShare.recvSelfRwd"
	RPCFrdShareRecvShareRwd                     RPCName = "frdShare.recvShareRwd"
	RPCFrdStealGetFrdStealRcdList               RPCName = "frdSteal.getFrdStealRcdList"
	RPCFrdStealGetStealStateByUids              RPCName = "frdSteal.getStealStateByUids"
	RPCFrdStealSteal                            RPCName = "frdSteal.steal"
	RPCFrdStealStealOneKey                      RPCName = "frdSteal.stealOneKey"
	RPCFreeWaterRecv                            RPCName = "freeWater.recv"
	RPCGameClubEnter                            RPCName = "gameClub.enter"
	RPCGameClubEnterClub                        RPCName = "gameClub.enterClub"
	RPCGameClubRecvTaskRwd                      RPCName = "gameClub.recvTaskRwd"
	RPCGirlsDayApply                            RPCName = "girlsDay.apply"
	RPCGirlsDayBind                             RPCName = "girlsDay.bind"
	RPCGirlsDayEnter                            RPCName = "girlsDay.enter"
	RPCGirlsDayFrdStates                        RPCName = "girlsDay.frdStates"
	RPCGirlsDayRecv                             RPCName = "girlsDay.recv"
	RPCGirlsDayReject                           RPCName = "girlsDay.reject"
	RPCGirlsDayUnBind                           RPCName = "girlsDay.unBind"
	RPCGiveGiftGetGiveUidList                   RPCName = "giveGift.getGiveUidList"
	RPCHomeRqstShowBird                         RPCName = "homeRqst.showBird"
	RPCIcoFrameActiveIcoFrame                   RPCName = "icoFrame.activeIcoFrame"
	RPCIcoFrameChgIcoFrame                      RPCName = "icoFrame.chgIcoFrame"
	RPCImChangeChannel                          RPCName = "im.changeChannel"
	RPCImDelChatPri                             RPCName = "im.delChatPri"
	RPCImDelChatPriRed                          RPCName = "im.delChatPriRed"
	RPCImEnter                                  RPCName = "im.enter"
	RPCImGetChannelId                           RPCName = "im.getChannelId"
	RPCImRead                                   RPCName = "im.read"
	RPCImRefuseStranger                         RPCName = "im.refuseStranger"
	RPCImSend                                   RPCName = "im.send"
	RPCIndexCreateUsr                           RPCName = "index.createUsr"
	RPCIndexLogin                               RPCName = "index.login"
	RPCIndexReLogin                             RPCName = "index.reLogin"
	RPCMailDel                                  RPCName = "mail.del"
	RPCMailDelOneKey                            RPCName = "mail.delOneKey"
	RPCMailGetList                              RPCName = "mail.getList"
	RPCMailOper                                 RPCName = "mail.oper"
	RPCMailPick                                 RPCName = "mail.pick"
	RPCMailPickOneKey                           RPCName = "mail.pickOneKey"
	RPCMailRead                                 RPCName = "mail.read"
	RPCMiniGameEndMiniGame                      RPCName = "miniGame.endMiniGame"
	RPCMiniGameEnterMiniGame                    RPCName = "miniGame.enterMiniGame"
	RPCMiniGameStartMiniGame                    RPCName = "miniGame.startMiniGame"
	RPCMiscBuyMonthCard                         RPCName = "misc.buyMonthCard"
	RPCMiscGetAdvanceWashItem                   RPCName = "misc.getAdvanceWashItem"
	RPCMiscRecvMsgPushRwd                       RPCName = "misc.recvMsgPushRwd"
	RPCMiscReportCheckBw                        RPCName = "misc.reportCheckBw"
	RPCMiscSellFlower                           RPCName = "misc.sellFlower"
	RPCMiscSyncItemHide                         RPCName = "misc.syncItemHide"
	RPCMonthFlowerBuy                           RPCName = "monthFlower.buy"
	RPCMonthFlowerEnter                         RPCName = "monthFlower.enter"
	RPCOpptGetDetailOppts                       RPCName = "oppt.getDetailOppts"
	RPCOpptGetOppt                              RPCName = "oppt.getOppt"
	RPCOpptGetOppts                             RPCName = "oppt.getOppts"
	RPCOrderCustomerFinishOrder                 RPCName = "orderCustomer.finishOrder"
	RPCOrderCustomerGenOrder                    RPCName = "orderCustomer.genOrder"
	RPCOrderCustomerRejectOrder                 RPCName = "orderCustomer.rejectOrder"
	RPCOrderFlowerEnter                         RPCName = "orderFlower.enter"
	RPCOrderFlowerFinishDecorateOrder           RPCName = "orderFlower.finishDecorateOrder"
	RPCOrderFlowerFinishOrder                   RPCName = "orderFlower.finishOrder"
	RPCOrderFlowerFinishSatinOrder              RPCName = "orderFlower.finishSatinOrder"
	RPCOrderFlowerRecvOrderRwd                  RPCName = "orderFlower.recvOrderRwd"
	RPCOrderFlowerRessuieOrderRwd               RPCName = "orderFlower.ressuieOrderRwd"
	RPCOrderPalaceEnter                         RPCName = "orderPalace.enter"
	RPCOrderPalaceFinishOrder                   RPCName = "orderPalace.finishOrder"
	RPCOrderPalaceGetOrderRcdList               RPCName = "orderPalace.getOrderRcdList"
	RPCOrderPalaceRefreshOrder                  RPCName = "orderPalace.refreshOrder"
	RPCOrderTeamRecvRwd                         RPCName = "orderTeam.recvRwd"
	RPCOrderTeamRefreshOrder                    RPCName = "orderTeam.refreshOrder"
	RPCOrderTeamStoreOrder                      RPCName = "orderTeam.storeOrder"
	RPCOrderTeamSubmitOrder                     RPCName = "orderTeam.submitOrder"
	RPCOrderTeamTakeOrder                       RPCName = "orderTeam.takeOrder"
	RPCOrderTeamTakeStoredOrder                 RPCName = "orderTeam.takeStoredOrder"
	RPCPearlDraw                                RPCName = "pearl.draw"
	RPCPearlGetHireMyLog                        RPCName = "pearl.getHireMyLog"
	RPCPearlGetHireStateByUids                  RPCName = "pearl.getHireStateByUids"
	RPCPearlGetMyHireLog                        RPCName = "pearl.getMyHireLog"
	RPCPearlGetRecommendList                    RPCName = "pearl.getRecommendList"
	RPCPearlRecvDailyFree                       RPCName = "pearl.recvDailyFree"
	RPCPearlRefresh                             RPCName = "pearl.refresh"
	RPCPearlSetProtectState                     RPCName = "pearl.setProtectState"
	RPCPearlPlaceHire                           RPCName = "pearlPlace.hire"
	RPCPearlPlaceRecv                           RPCName = "pearlPlace.recv"
	RPCPearlPlaceRecvOneKey                     RPCName = "pearlPlace.recvOneKey"
	RPCPearlPlaceUnlockPlace                    RPCName = "pearlPlace.unlockPlace"
	RPCPhotoBuy                                 RPCName = "photo.buy"
	RPCPhotoBuyTicket                           RPCName = "photo.buyTicket"
	RPCPhotoCheckInvite                         RPCName = "photo.checkInvite"
	RPCPhotoCloseRoom                           RPCName = "photo.closeRoom"
	RPCPhotoDelRoomUsr                          RPCName = "photo.delRoomUsr"
	RPCPhotoEnter                               RPCName = "photo.enter"
	RPCPhotoEnterRoom                           RPCName = "photo.enterRoom"
	RPCPhotoFinishRoom                          RPCName = "photo.finishRoom"
	RPCPhotoGetBase64                           RPCName = "photo.getBase64"
	RPCPhotoGetFriendList                       RPCName = "photo.getFriendList"
	RPCPhotoGetInviteList                       RPCName = "photo.getInviteList"
	RPCPhotoGetPhotoList                        RPCName = "photo.getPhotoList"
	RPCPhotoInvite                              RPCName = "photo.invite"
	RPCPhotoInviteDeal                          RPCName = "photo.inviteDeal"
	RPCPhotoPushBase64                          RPCName = "photo.pushBase64"
	RPCPhotoReadInvite                          RPCName = "photo.readInvite"
	RPCPhotoReadPhoto                           RPCName = "photo.readPhoto"
	RPCPhotoReadRoomMsg                         RPCName = "photo.readRoomMsg"
	RPCPhotoReclaimRoom                         RPCName = "photo.reclaimRoom"
	RPCPhotoRejectInvite                        RPCName = "photo.rejectInvite"
	RPCPhotoReshoot                             RPCName = "photo.reshoot"
	RPCPhotoSavePhoto                           RPCName = "photo.savePhoto"
	RPCPhotoSaveRoomPhoto                       RPCName = "photo.saveRoomPhoto"
	RPCPhotoSaveRoomPro                         RPCName = "photo.saveRoomPro"
	RPCPhotoSaveRoomUsr                         RPCName = "photo.saveRoomUsr"
	RPCPhotoSetPhotoStatus                      RPCName = "photo.setPhotoStatus"
	RPCPhotoSetRefuseInvite                     RPCName = "photo.setRefuseInvite"
	RPCPhotoTakePhoto                           RPCName = "photo.takePhoto"
	RPCPhotoTransmitRoom                        RPCName = "photo.transmitRoom"
	RPCPlayerBackPlayerBackPassEnter            RPCName = "playerBack.playerBackPassEnter"
	RPCPlayerBackPlayerBackPassRecv             RPCName = "playerBack.playerBackPassRecv"
	RPCPlayerBackPlayerBackPassRecvOneKey       RPCName = "playerBack.playerBackPassRecvOneKey"
	RPCPlayerBackPlayerBackPassTaskDone         RPCName = "playerBack.playerBackPassTaskDone"
	RPCPlayerBackSign                           RPCName = "playerBack.sign"
	RPCPlayerBackSignEnter                      RPCName = "playerBack.signEnter"
	RPCPlayerBackSignRecv                       RPCName = "playerBack.signRecv"
	RPCPlayerBackUpdateGuildIds                 RPCName = "playerBack.updateGuildIds"
	RPCRandomEventDoAffair                      RPCName = "randomEvent.doAffair"
	RPCRandomEventEnter                         RPCName = "randomEvent.enter"
	RPCRankGetRanks                             RPCName = "rank.getRanks"
	RPCRchgCardRecv                             RPCName = "rchgCard.recv"
	RPCRchgDayEnter                             RPCName = "rchgDay.enter"
	RPCRchgDayReceive                           RPCName = "rchgDay.receive"
	RPCRchgOrderToMoneyConvertMoney             RPCName = "rchgOrderToMoney.convertMoney"
	RPCRchgSumRecv                              RPCName = "rchgSum.recv"
	RPCRedeemGetInfo                            RPCName = "redeem.getInfo"
	RPCRedeemUseCode                            RPCName = "redeem.useCode"
	RPCRedeemCodeShowDjdk                       RPCName = "redeemCodeShow.djdk"
	RPCReputationAppeal                         RPCName = "reputation.appeal"
	RPCReputationGetLogs                        RPCName = "reputation.getLogs"
	RPCReputationView                           RPCName = "reputation.view"
	RPCReserveCheckRwd                          RPCName = "reserve.checkRwd"
	RPCRoadGrowRecv                             RPCName = "roadGrow.recv"
	RPCRoadGrowRecvBox                          RPCName = "roadGrow.recvBox"
	RPCRwdRecv                                  RPCName = "rwd.recv"
	RPCRwdSetCanRecv                            RPCName = "rwd.setCanRecv"
	RPCSdkCheckRchg                             RPCName = "sdk.checkRchg"
	RPCSdkMoniPay                               RPCName = "sdk.moniPay"
	RPCSdkPayByMoney                            RPCName = "sdk.payByMoney"
	RPCSdkSendGoods                             RPCName = "sdk.sendGoods"
	RPCSecPwdChangePwd                          RPCName = "secPwd.changePwd"
	RPCSecPwdCheckPwd                           RPCName = "secPwd.checkPwd"
	RPCSecPwdCloseSecPwd                        RPCName = "secPwd.closeSecPwd"
	RPCSecPwdFirstUse                           RPCName = "secPwd.firstUse"
	RPCSecPwdGetQuestion                        RPCName = "secPwd.getQuestion"
	RPCSecPwdResetPwd                           RPCName = "secPwd.resetPwd"
	RPCSecPwdSetPwd                             RPCName = "secPwd.setPwd"
	RPCShopBuy                                  RPCName = "shop.buy"
	RPCShopEnter                                RPCName = "shop.enter"
	RPCShopRefresh                              RPCName = "shop.refresh"
	RPCShopSync                                 RPCName = "shop.sync"
	RPCShopCultivateBuy                         RPCName = "shopCultivate.buy"
	RPCShopCultivateBuyOneKey                   RPCName = "shopCultivate.buyOneKey"
	RPCShopCultivateEnter                       RPCName = "shopCultivate.enter"
	RPCShopCultivateRefresh                     RPCName = "shopCultivate.refresh"
	RPCShopFlowerElvesBuy                       RPCName = "shopFlowerElves.buy"
	RPCShopFlowerElvesEnter                     RPCName = "shopFlowerElves.enter"
	RPCShopFmlRaceBuy                           RPCName = "shopFmlRace.buy"
	RPCShopFmlUsrBuildShop                      RPCName = "shopFmlUsr.buildShop"
	RPCShopFmlUsrBuy                            RPCName = "shopFmlUsr.buy"
	RPCShopFmlUsrBuyAll                         RPCName = "shopFmlUsr.buyAll"
	RPCShopFmlUsrEnter                          RPCName = "shopFmlUsr.enter"
	RPCShopFmlUsrRefresh                        RPCName = "shopFmlUsr.refresh"
	RPCShopFmlUsrUnlockSlot                     RPCName = "shopFmlUsr.unlockSlot"
	RPCShopGiftbagBuy                           RPCName = "shopGiftbag.buy"
	RPCShopGiftbagEnter                         RPCName = "shopGiftbag.enter"
	RPCSignRecvGradeRwd                         RPCName = "sign.recvGradeRwd"
	RPCSignSign                                 RPCName = "sign.sign"
	RPCSignSignSeven                            RPCName = "sign.sign_seven"
	RPCSignTypeEnter                            RPCName = "signType.enter"
	RPCSignTypeRecv                             RPCName = "signType.recv"
	RPCSignTypeSign                             RPCName = "signType.sign"
	RPCStoryMainEnter                           RPCName = "storyMain.enter"
	RPCStoryMainUnlock                          RPCName = "storyMain.unlock"
	RPCSysInformChat                            RPCName = "sys.informChat"
	RPCSysInformFml                             RPCName = "sys.informFml"
	RPCSysInformUsr                             RPCName = "sys.informUsr"
	RPCTaskAchRecv                              RPCName = "taskAch.recv"
	RPCTaskAchRecvOneKey                        RPCName = "taskAch.recvOneKey"
	RPCTaskDlyEnter                             RPCName = "taskDly.enter"
	RPCTaskDlyRecv                              RPCName = "taskDly.recv"
	RPCTaskDlyRecvBox                           RPCName = "taskDly.recvBox"
	RPCTaskInvRecv                              RPCName = "taskInv.recv"
	RPCTaskInvRecvOneKey                        RPCName = "taskInv.recvOneKey"
	RPCTaskMainRecv                             RPCName = "taskMain.recv"
	RPCTaskSysGiftBuy                           RPCName = "taskSys.giftBuy"
	RPCTaskSysRecv                              RPCName = "taskSys.recv"
	RPCTaskSysRecvLvlRwd                        RPCName = "taskSys.recvLvlRwd"
	RPCTaskSysRecvOneKey                        RPCName = "taskSys.recvOneKey"
	RPCTaskWeekRecv                             RPCName = "taskWeek.recv"
	RPCTeamOrderPopupShowT                      RPCName = "teamOrderPopup.showT"
	RPCThirdpartyApplyToken                     RPCName = "thirdparty.applyToken"
	RPCTitleActiveTitle                         RPCName = "title.activeTitle"
	RPCTitleChgTitle                            RPCName = "title.chgTitle"
	RPCTitleSetTitleShow                        RPCName = "title.setTitleShow"
	RPCTokenInfoGetToken                        RPCName = "tokenInfo.getToken"
	RPCTtMoneyTaskGenGldOrder                   RPCName = "ttMoneyTask.genGldOrder"
	RPCTtMoneyTaskRecv                          RPCName = "ttMoneyTask.recv"
	RPCTtMoneyTaskRefresh                       RPCName = "ttMoneyTask.refresh"
	RPCUsrActiveCard                            RPCName = "usr.activeCard"
	RPCUsrActiveEmoji                           RPCName = "usr.activeEmoji"
	RPCUsrActiveHead                            RPCName = "usr.activeHead"
	RPCUsrActiveMedal                           RPCName = "usr.activeMedal"
	RPCUsrAfterShare                            RPCName = "usr.afterShare"
	RPCUsrChgCard                               RPCName = "usr.chgCard"
	RPCUsrChgFace                               RPCName = "usr.chgFace"
	RPCUsrChgIco                                RPCName = "usr.chgIco"
	RPCUsrChgName                               RPCName = "usr.chgName"
	RPCUsrChgSex                                RPCName = "usr.chgSex"
	RPCUsrChgSign                               RPCName = "usr.chgSign"
	RPCUsrClearVipService                       RPCName = "usr.clearVipService"
	RPCUsrGetSalary                             RPCName = "usr.getSalary"
	RPCUsrHeartTick                             RPCName = "usr.heartTick"
	RPCUsrLazySync                              RPCName = "usr.lazySync"
	RPCUsrRecvSignRwd                           RPCName = "usr.recvSignRwd"
	RPCUsrRefreshMedal                          RPCName = "usr.refreshMedal"
	RPCUsrSaveAuthInfo                          RPCName = "usr.saveAuthInfo"
	RPCUsrSaveCustIco                           RPCName = "usr.saveCustIco"
	RPCUsrSetMedalShow                          RPCName = "usr.setMedalShow"
	RPCUsrShare                                 RPCName = "usr.share"
	RPCUsrTriggerEvent                          RPCName = "usr.triggerEvent"
	RPCUsrUpdateGuide                           RPCName = "usr.updateGuide"
	RPCUsrUpdateSoftGuide                       RPCName = "usr.updateSoftGuide"
	RPCUsrUpdateUsrExt                          RPCName = "usr.updateUsrExt"
	RPCUsrUpdateUsrSet                          RPCName = "usr.updateUsrSet"
	RPCUsrUpdateVipService                      RPCName = "usr.updateVipService"
	RPCUsrUpgrade                               RPCName = "usr.upgrade"
	RPCUsrWorship                               RPCName = "usr.worship"
	RPCUsrExtraRecvAntiFraudQARwd               RPCName = "usrExtra.recvAntiFraudQARwd"
	RPCUsrExtraRecvTtFansRwd                    RPCName = "usrExtra.recvTtFansRwd"
	RPCUsrExtraRecvVersionUpdateRwd             RPCName = "usrExtra.recvVersionUpdateRwd"
	RPCUsrExtraRecvWbFansRwd                    RPCName = "usrExtra.recvWbFansRwd"
	RPCUsrExtraReportUsr                        RPCName = "usrExtra.reportUsr"
	RPCUsrExtraSetMsPwd                         RPCName = "usrExtra.setMsPwd"
	RPCUsrExtraSetShowAddress                   RPCName = "usrExtra.setShowAddress"
	RPCUsrExtraShareMsg                         RPCName = "usrExtra.shareMsg"
	RPCUsrExtraSyncAddress                      RPCName = "usrExtra.syncAddress"
	RPCUsrExtraUpdateAntiFraudQAStatus          RPCName = "usrExtra.updateAntiFraudQAStatus"
	RPCUsrExtraUpdateTaskMap                    RPCName = "usrExtra.updateTaskMap"
	RPCUsrExtraUpdateTtSubscribe                RPCName = "usrExtra.updateTtSubscribe"
	RPCUsrLandClear                             RPCName = "usrLand.clear"
	RPCUsrLandClearBatch                        RPCName = "usrLand.clearBatch"
	RPCUsrLandClearOneKey                       RPCName = "usrLand.clearOneKey"
	RPCUsrLandHarvest                           RPCName = "usrLand.harvest"
	RPCUsrLandHarvestOneKey                     RPCName = "usrLand.harvestOneKey"
	RPCUsrLandPlant                             RPCName = "usrLand.plant"
	RPCUsrLandPlantBatch                        RPCName = "usrLand.plantBatch"
	RPCUsrLandPlantOneKey                       RPCName = "usrLand.plantOneKey"
	RPCUsrLandRefresh                           RPCName = "usrLand.refresh"
	RPCUsrLandSpeedUp                           RPCName = "usrLand.speedUp"
	RPCUsrLandSpeedUpBatch                      RPCName = "usrLand.speedUpBatch"
	RPCUsrLandSpeedUpFree                       RPCName = "usrLand.speedUpFree"
	RPCUsrLandSpeedUpOneKey                     RPCName = "usrLand.speedUpOneKey"
	RPCUsrLandUnlockLand                        RPCName = "usrLand.unlockLand"
	RPCUsrLandWater                             RPCName = "usrLand.water"
	RPCUsrLandWaterBatch                        RPCName = "usrLand.waterBatch"
	RPCUsrLandWaterOneKey                       RPCName = "usrLand.waterOneKey"
	RPCUsrRedDelRed                             RPCName = "usrRed.delRed"
	RPCUsrSubscribePushAddSubscribeNum          RPCName = "usrSubscribePush.addSubscribeNum"
	RPCUsrSubscribePushAddSubscribeNumPermanent RPCName = "usrSubscribePush.addSubscribeNumPermanent"
	RPCUsrSubscribePushMsgPushSetting           RPCName = "usrSubscribePush.msgPushSetting"
	RPCUsrSubscribePushMsgPushSettingGlobal     RPCName = "usrSubscribePush.msgPushSettingGlobal"
	RPCUsrVerInfoRefresh                        RPCName = "usrVerInfo.refresh"
	RPCValentinesApply                          RPCName = "valentines.apply"
	RPCValentinesBind                           RPCName = "valentines.bind"
	RPCValentinesEnter                          RPCName = "valentines.enter"
	RPCValentinesFrdStates                      RPCName = "valentines.frdStates"
	RPCValentinesRecv                           RPCName = "valentines.recv"
	RPCValentinesReject                         RPCName = "valentines.reject"
	RPCValentinesUnBind                         RPCName = "valentines.unBind"
	RPCVerifyCheckVerification                  RPCName = "verify.checkVerification"
	RPCVerifyRefreshVerification                RPCName = "verify.refreshVerification"
	RPCVipRecv                                  RPCName = "vip.recv"
	RPCWaterRqstDjst                            RPCName = "waterRqst.djst"
	RPCWaterwheelEnter                          RPCName = "waterwheel.enter"
	RPCWaterwheelRecv                           RPCName = "waterwheel.recv"
	RPCWaterwheelSkip                           RPCName = "waterwheel.skip"
	RPCWhiteDay26Apply                          RPCName = "whiteDay26.apply"
	RPCWhiteDay26Bind                           RPCName = "whiteDay26.bind"
	RPCWhiteDay26Enter                          RPCName = "whiteDay26.enter"
	RPCWhiteDay26FrdStates                      RPCName = "whiteDay26.frdStates"
	RPCWhiteDay26Recv                           RPCName = "whiteDay26.recv"
	RPCWhiteDay26Reject                         RPCName = "whiteDay26.reject"
	RPCWhiteDay26UnBind                         RPCName = "whiteDay26.unBind"
	RPCWhiteValentineApply                      RPCName = "whiteValentine.apply"
	RPCWhiteValentineBind                       RPCName = "whiteValentine.bind"
	RPCWhiteValentineEnter                      RPCName = "whiteValentine.enter"
	RPCWhiteValentineFrdStates                  RPCName = "whiteValentine.frdStates"
	RPCWhiteValentineRecv                       RPCName = "whiteValentine.recv"
	RPCWhiteValentineReject                     RPCName = "whiteValentine.reject"
	RPCWhiteValentineUnBind                     RPCName = "whiteValentine.unBind"
	RPCZooAddFoodstuff                          RPCName = "zoo.addFoodstuff"
	RPCZooCalNaturalAtt                         RPCName = "zoo.calNaturalAtt"
	RPCZooChangePetName                         RPCName = "zoo.changePetName"
	RPCZooEnterZoo                              RPCName = "zoo.enterZoo"
	RPCZooFeedOtherPet                          RPCName = "zoo.feedOtherPet"
	RPCZooFeedPets                              RPCName = "zoo.feedPets"
	RPCZooFindPet                               RPCName = "zoo.findPet"
	RPCZooFindPetByUsrBack                      RPCName = "zoo.findPetByUsrBack"
	RPCZooGetGuideEventRwd                      RPCName = "zoo.getGuideEventRwd"
	RPCZooHandBeOverdueEvent                    RPCName = "zoo.handBeOverdueEvent"
	RPCZooHandleEvent                           RPCName = "zoo.handleEvent"
	RPCZooInitZoo                               RPCName = "zoo.initZoo"
	RPCZooReadLog                               RPCName = "zoo.readLog"
	RPCZooReadSouvenir                          RPCName = "zoo.readSouvenir"
	RPCZooRecvSouvenirRwd                       RPCName = "zoo.recvSouvenirRwd"
	RPCZooRefreshPetStatus                      RPCName = "zoo.refreshPetStatus"
	RPCZooSetUpSleepTime                        RPCName = "zoo.setUpSleepTime"
	RPCZooStrokePet                             RPCName = "zoo.strokePet"
	RPCZooUsePetItem                            RPCName = "zoo.usePetItem"
	RPCZooVisitZoo                              RPCName = "zoo.visitZoo"
	RPCZooDecorateEquip                         RPCName = "zooDecorate.equip"
	RPCZooDecorateRead                          RPCName = "zooDecorate.read"
)

var gameJSRPCNames = []RPCName{
	"PlantRqst.zhtc",
	"ReapPopup.shjm",
	"act.buy",
	"act.getOneOrderAward",
	"act.getOrderAward",
	"act.getRankAward",
	"act.getStat",
	"act.giftBuy",
	"act.recv",
	"act.recvBoxes",
	"act.recvTLAward",
	"act.refreshDailyGift",
	"act.refreshTask",
	"act.syncBatchInfo",
	"actCallBack.actCallBackBind",
	"actCallBack.actCallBackEnter",
	"actCallBack.actCallBackRecv",
	"actCardCollect.checkLuckyCardSend",
	"actCardCollect.deckShopExchange",
	"actCardCollect.nextRound",
	"actCardCollect.openCardPack",
	"actCardCollect.recvBoxReward",
	"actCardCollect.recvCardAlbumReward",
	"actCardCollect.recvCollectReward",
	"actCardCollect.recvTaskReward",
	"actCardCollect.refreshTaskData",
	"actCardCollect.useSelectedCard",
	"actCyclicNote.directRecvTaskRwd",
	"actCyclicNote.giftBuy",
	"actCyclicNote.reRandomTask",
	"actCyclicNote.recv",
	"actCyclicNote.recvTaskRwd",
	"actCyclicNote.resetGiftCd",
	"actCyclicNote.unlockTaskSlot",
	"actCyclicStory.giftBuy",
	"actCyclicStory.reRandomOrder",
	"actCyclicStory.recv",
	"actCyclicStory.recvOrderRwd",
	"actCyclicStory.removeOrderCd",
	"actCyclicStory.resetGiftCd",
	"actCyclicVase.giftBuy",
	"actCyclicVase.recv",
	"actCyclicVase.resetGiftCd",
	"actDessert.enter",
	"actDessert.gameOver",
	"actDessert.gameStart",
	"actDessert.gameSync",
	"actDessert.giftBuy",
	"actDessert.openBox",
	"actDraw.draw",
	"actDrawChristmas.draw",
	"actDrawChristmas.enter",
	"actDrawChristmas.giftBuy",
	"actDrawDragon.draw",
	"actDrawDragon.giftBuy",
	"actDrawDragon.recv",
	"actDrawGift.giftBuy",
	"actDrawSprSkin.draw",
	"actDrawSprSkin.enter",
	"actDrawSprSkin.giftBuy",
	"actDrawZb.draw",
	"actDrawZb.enter",
	"actDrawZb.giftBuy",
	"actElim.enter",
	"actElim.giftBuy",
	"actElim.move",
	"actElim.openBox",
	"actElim.refreshMap",
	"actElim.useItem1",
	"actElim.useItem2",
	"actFlowerBattle.chooseFlowerArt",
	"actFlowerBattle.enter",
	"actFlowerBattle.getGiftBuyRecords",
	"actFlowerBattle.giftBuy",
	"actFlowerBattle.like",
	"actFlowerBattle.recvBoxesPrize",
	"actFlowerBattle.setIsAnonymous",
	"actFmlRedEnvelope.enter",
	"actFmlRedEnvelope.getDetail",
	"actFmlRedEnvelope.getRecord",
	"actFmlRedEnvelope.list",
	"actFmlRedEnvelope.pick",
	"actFmlRedEnvelope.send",
	"actGame2048.enter",
	"actGame2048.giftBuy",
	"actGame2048.move",
	"actGame2048.openBox",
	"actGame2048.restart",
	"actGame2048.useChange",
	"actGame2048.useEliminate",
	"actHoney.giftBuy",
	"actHoney.recv",
	"actHoney.resetGiftCd",
	"actIPDmdGift.giftBuy",
	"actIPFlowerGuard.openBox",
	"actMerge2.enter",
	"actMerge2.move",
	"actMerge2.openBox",
	"actMerge2.putInWarehouse",
	"actMerge2.putOutTemp",
	"actMerge2.putOutWarehouse",
	"actMerge2.recvOrder",
	"actMerge2.recvProgress",
	"actMerge2.refreshOrder",
	"actMerge2.saveGuide",
	"actMerge2.sellItem",
	"actMerge2.splitItem",
	"actMerge2.switchMode",
	"actMerge2.unlockWarehouse",
	"actMerge2.useItem",
	"actOfficials.buyItem",
	"actOfficials.enter",
	"actOfficials.recvGrpReachPrize",
	"actOfficials.useItem",
	"actPaper.enter",
	"actPaper.recv",
	"actPaper.recvGamePrize",
	"actPaper.recvTaskPrize",
	"actRchgRwd.enter",
	"actRchgRwd.recv",
	"actRchgWheel.enter",
	"actRchgWheel.getMyLog",
	"actRchgWheel.startWheel",
	"actSpool.enter",
	"actSpool.gameOver",
	"actSpool.gameStart",
	"actSpool.gameSync",
	"actSpool.giftBuy",
	"actSpool.openBox",
	"actSpool.rise",
	"actSpool.setGuideStatus",
	"actSpringTotRchg.recvTLAward",
	"actVipTimeShop.giftBuy",
	"actZFBForest.browseWeb",
	"actZFBForest.browseWeb2",
	"bag.combine",
	"bag.sell",
	"bag.use",
	"battlePass.buyLvl",
	"battlePass.recv",
	"battlePass.recvAll",
	"battlePass.taskDone",
	"benefitBox.draw",
	"bestie.apply",
	"bestie.cancelDissolve",
	"bestie.checkApply",
	"bestie.dissolve",
	"bestie.enter",
	"bestie.getFrdBestieCntMap",
	"bestie.handleApply",
	"bestie.immediateDissolve",
	"bestie.setSceneSkin",
	"bestie.unlockSlot",
	"boost.recvRwd",
	"boost.refresh",
	"bubble.activeBubble",
	"bubble.chgBubble",
	"callFriend.enter",
	"callFriend.recv",
	"callFriend.useCode",
	"celebrity.getAllTypes",
	"celebrity.getAllTypesInfo",
	"celebrity.getInfoByType",
	"celebrity.likeCelebrity",
	"channelRwd.recvDailyDesktopRwd",
	"channelRwd.recvFstDesktopRwd",
	"channelRwd.recvFstSidebarRwd",
	"channelRwd.recvLoginRwd",
	"cheater.doCheat",
	"collectRwd.recv",
	"collectRwd.recvArtCreateRwd",
	"collectRwd.recvArtCreateRwdByVase",
	"cultivate.chooseSkill",
	"cultivate.clearCulCd",
	"cultivate.cultivate",
	"cultivate.randomSkill",
	"cultivate.recv",
	"cultivate.reduceByHelp",
	"cultivate.reduceByItem",
	"cultivate.unlockSlot",
	"cultivate.upgrade",
	"customerOrderRqst.dkgkck",
	"decorate.build",
	"decorate.buildSuccess",
	"decorate.clearBuildCd",
	"decorate.equip",
	"decorate.recv",
	"decorate.updateReadLvlList",
	"draw.draw",
	"draw.testDrawVirtual",
	"fashion.equip",
	"fashion.read",
	"flowerArt.makeFlowerArt",
	"flowerElves.checkConvert",
	"flowerElvesAid.helpFrd",
	"flowerElvesAid.recvAidEff",
	"flowerElvesAid.reqAid",
	"flowerElvesBook.upgrade",
	"flowerElvesBookDraw.draw",
	"flowerElvesBookDraw.refresh",
	"flowerElvesPlace.dispatch",
	"flowerElvesPlace.recv",
	"flowerElvesPlace.recvAllReward",
	"flowerElvesPlace.speedUp",
	"flowerElvesPlace.unlock",
	"flowerGift.recvBox",
	"flowerMarket.buyFlower",
	"flowerMarket.buyPutCount",
	"flowerMarket.checkPassword",
	"flowerMarket.enter",
	"flowerMarket.getFriend",
	"flowerMarket.getFriendList",
	"flowerMarket.getTradeRecords",
	"flowerMarket.harvestIncome",
	"flowerMarket.putFlower",
	"flowerMarket.putFlowerBatch",
	"flowerMarket.takeDownFlower",
	"flowerMarket.unlockShelf",
	"flowerOrderRqst.showR",
	"flowerRack.cancelSell",
	"flowerRack.recvOneKey",
	"flowerRack.recvSellMoney",
	"flowerRack.sell",
	"flowerRack.unlockStand",
	"fml.autoJoin",
	"fml.bld",
	"fml.buyRaceBoat",
	"fml.chgPos",
	"fml.chgTitle",
	"fml.clearQuitTime",
	"fml.create",
	"fml.dissolve",
	"fml.enter",
	"fml.equipRaceBoat",
	"fml.getHonor",
	"fml.getLog",
	"fml.getMedalRwd",
	"fml.getRecFmlList",
	"fml.getTitleLogList",
	"fml.handleApply",
	"fml.handleApplyAll",
	"fml.handleInv",
	"fml.inv",
	"fml.join",
	"fml.kick",
	"fml.openFmlRaceBox",
	"fml.quit",
	"fml.recvBox",
	"fml.refreshRaceBoat",
	"fml.refreshTitle",
	"fml.search",
	"fml.setting",
	"fml.unbindUnionGroup",
	"fml.unloadRaceBoat",
	"fml.upgradeFml",
	"fml.upgradeRaceBoat",
	"fmlFlowerShare.addTakeCnt",
	"fmlFlowerShare.getFmlOtherShareList",
	"fmlFlowerShare.getShareLogList",
	"fmlFlowerShare.recvRwd",
	"fmlFlowerShare.refresh",
	"fmlFlowerShare.share",
	"fmlFlowerShare.take",
	"fmlFlowerShare.unlock",
	"fmlFlowerShow.cancelLikeOther",
	"fmlFlowerShow.getLikeOtherRecord",
	"fmlFlowerShow.getLikeOtherRecord5Limit",
	"fmlFlowerShow.getShowInfo",
	"fmlFlowerShow.likeOther",
	"fmlFlowerShow.saveShow",
	"fmlFlowerShow.setVisitType",
	"fmlFlowerShow.switchMap",
	"fmlFlowerShow.unlockSlot",
	"fmlForest.applyPlant",
	"fmlForest.collectEnergy",
	"fmlForest.enter",
	"fmlForest.getCertDetail",
	"fmlForest.getCertDetailByFid",
	"fmlForest.getLogList",
	"fmlForest.getTreeList",
	"fmlForest.getWeekCollect",
	"fmlForest.getWeekStat",
	"fmlForest.refresh",
	"fmlLand.harvest",
	"fmlLand.harvestAll",
	"fmlLand.plant",
	"fmlLand.unlock",
	"fmlLand.upgrade",
	"fmlRace.buyTaskNum",
	"fmlRace.delTask",
	"fmlRace.enter",
	"fmlRace.finishTask",
	"fmlRace.getFmlRaceEndDisplayData",
	"fmlRace.getFmlRaceHistRcdList",
	"fmlRace.getFmlRaceTaskScore",
	"fmlRace.getFmlRaceUsrRankList",
	"fmlRace.getGroupFmlRaceRcdList",
	"fmlRace.getNewMbScoreRank",
	"fmlRace.getTaskList",
	"fmlRace.getTaskLogList",
	"fmlRace.giveUpTask",
	"fmlRace.refreshFmlRaceBatch",
	"fmlRace.refreshFmlRaceBox",
	"fmlRace.refreshTask",
	"fmlRace.takeTask",
	"fmlRace.upgradeTask",
	"fmlRaceRqst.showShip",
	"fmlTaskEnter.showtcrw",
	"frd.addBlack",
	"frd.applyFrd",
	"frd.applyFrdBatch",
	"frd.del",
	"frd.delBlack",
	"frd.enter",
	"frd.getApplyList",
	"frd.getBlackList",
	"frd.getFriendList",
	"frd.handleApply",
	"frd.handleApplyBatch",
	"frd.like",
	"frd.refreshRecList",
	"frd.setFrdRjt",
	"frdExt.buyStealCnt",
	"frdExt.cancelFollow",
	"frdExt.follow",
	"frdExt.getFrdOtherInfoByUids",
	"frdExt.searchUser",
	"frdHome.getFrdHomeInfo",
	"frdShare.enter",
	"frdShare.recvBoxRwd",
	"frdShare.recvSelfRwd",
	"frdShare.recvShareRwd",
	"frdSteal.getFrdStealRcdList",
	"frdSteal.getStealStateByUids",
	"frdSteal.steal",
	"frdSteal.stealOneKey",
	"freeWater.recv",
	"gameClub.enter",
	"gameClub.enterClub",
	"gameClub.recvTaskRwd",
	"girlsDay.apply",
	"girlsDay.bind",
	"girlsDay.enter",
	"girlsDay.frdStates",
	"girlsDay.recv",
	"girlsDay.reject",
	"girlsDay.unBind",
	"giveGift.getGiveUidList",
	"homeRqst.showBird",
	"icoFrame.activeIcoFrame",
	"icoFrame.chgIcoFrame",
	"im.changeChannel",
	"im.delChatPri",
	"im.delChatPriRed",
	"im.enter",
	"im.getChannelId",
	"im.read",
	"im.refuseStranger",
	"im.send",
	"index.createUsr",
	"index.login",
	"index.reLogin",
	"mail.del",
	"mail.delOneKey",
	"mail.getList",
	"mail.oper",
	"mail.pick",
	"mail.pickOneKey",
	"mail.read",
	"miniGame.endMiniGame",
	"miniGame.enterMiniGame",
	"miniGame.startMiniGame",
	"misc.buyMonthCard",
	"misc.getAdvanceWashItem",
	"misc.recvMsgPushRwd",
	"misc.reportCheckBw",
	"misc.sellFlower",
	"misc.syncItemHide",
	"monthFlower.buy",
	"monthFlower.enter",
	"oppt.getDetailOppts",
	"oppt.getOppt",
	"oppt.getOppts",
	"orderCustomer.finishOrder",
	"orderCustomer.genOrder",
	"orderCustomer.rejectOrder",
	"orderFlower.enter",
	"orderFlower.finishDecorateOrder",
	"orderFlower.finishOrder",
	"orderFlower.finishSatinOrder",
	"orderFlower.recvOrderRwd",
	"orderFlower.ressuieOrderRwd",
	"orderPalace.enter",
	"orderPalace.finishOrder",
	"orderPalace.getOrderRcdList",
	"orderPalace.refreshOrder",
	"orderTeam.recvRwd",
	"orderTeam.refreshOrder",
	"orderTeam.storeOrder",
	"orderTeam.submitOrder",
	"orderTeam.takeOrder",
	"orderTeam.takeStoredOrder",
	"pearl.draw",
	"pearl.getHireMyLog",
	"pearl.getHireStateByUids",
	"pearl.getMyHireLog",
	"pearl.getRecommendList",
	"pearl.recvDailyFree",
	"pearl.refresh",
	"pearl.setProtectState",
	"pearlPlace.hire",
	"pearlPlace.recv",
	"pearlPlace.recvOneKey",
	"pearlPlace.unlockPlace",
	"photo.buy",
	"photo.buyTicket",
	"photo.checkInvite",
	"photo.closeRoom",
	"photo.delRoomUsr",
	"photo.enter",
	"photo.enterRoom",
	"photo.finishRoom",
	"photo.getBase64",
	"photo.getFriendList",
	"photo.getInviteList",
	"photo.getPhotoList",
	"photo.invite",
	"photo.inviteDeal",
	"photo.pushBase64",
	"photo.readInvite",
	"photo.readPhoto",
	"photo.readRoomMsg",
	"photo.reclaimRoom",
	"photo.rejectInvite",
	"photo.reshoot",
	"photo.savePhoto",
	"photo.saveRoomPhoto",
	"photo.saveRoomPro",
	"photo.saveRoomUsr",
	"photo.setPhotoStatus",
	"photo.setRefuseInvite",
	"photo.takePhoto",
	"photo.transmitRoom",
	"playerBack.playerBackPassEnter",
	"playerBack.playerBackPassRecv",
	"playerBack.playerBackPassRecvOneKey",
	"playerBack.playerBackPassTaskDone",
	"playerBack.sign",
	"playerBack.signEnter",
	"playerBack.signRecv",
	"playerBack.updateGuildIds",
	"randomEvent.doAffair",
	"randomEvent.enter",
	"rank.getRanks",
	"rchgCard.recv",
	"rchgDay.enter",
	"rchgDay.receive",
	"rchgOrderToMoney.convertMoney",
	"rchgSum.recv",
	"redeem.getInfo",
	"redeem.useCode",
	"redeemCodeShow.djdk",
	"reputation.appeal",
	"reputation.getLogs",
	"reputation.view",
	"reserve.checkRwd",
	"roadGrow.recv",
	"roadGrow.recvBox",
	"rwd.recv",
	"rwd.setCanRecv",
	"sdk.checkRchg",
	"sdk.moniPay",
	"sdk.payByMoney",
	"sdk.sendGoods",
	"secPwd.changePwd",
	"secPwd.checkPwd",
	"secPwd.closeSecPwd",
	"secPwd.firstUse",
	"secPwd.getQuestion",
	"secPwd.resetPwd",
	"secPwd.setPwd",
	"shop.buy",
	"shop.enter",
	"shop.refresh",
	"shop.sync",
	"shopCultivate.buy",
	"shopCultivate.buyOneKey",
	"shopCultivate.enter",
	"shopCultivate.refresh",
	"shopFlowerElves.buy",
	"shopFlowerElves.enter",
	"shopFmlRace.buy",
	"shopFmlUsr.buildShop",
	"shopFmlUsr.buy",
	"shopFmlUsr.buyAll",
	"shopFmlUsr.enter",
	"shopFmlUsr.refresh",
	"shopFmlUsr.unlockSlot",
	"shopGiftbag.buy",
	"shopGiftbag.enter",
	"sign.recvGradeRwd",
	"sign.sign",
	"sign.sign_seven",
	"signType.enter",
	"signType.recv",
	"signType.sign",
	"storyMain.enter",
	"storyMain.unlock",
	"sys.informChat",
	"sys.informFml",
	"sys.informUsr",
	"taskAch.recv",
	"taskAch.recvOneKey",
	"taskDly.enter",
	"taskDly.recv",
	"taskDly.recvBox",
	"taskInv.recv",
	"taskInv.recvOneKey",
	"taskMain.recv",
	"taskSys.giftBuy",
	"taskSys.recv",
	"taskSys.recvLvlRwd",
	"taskSys.recvOneKey",
	"taskWeek.recv",
	"teamOrderPopup.showT",
	"thirdparty.applyToken",
	"title.activeTitle",
	"title.chgTitle",
	"title.setTitleShow",
	"tokenInfo.getToken",
	"ttMoneyTask.genGldOrder",
	"ttMoneyTask.recv",
	"ttMoneyTask.refresh",
	"usr.activeCard",
	"usr.activeEmoji",
	"usr.activeHead",
	"usr.activeMedal",
	"usr.afterShare",
	"usr.chgCard",
	"usr.chgFace",
	"usr.chgIco",
	"usr.chgName",
	"usr.chgSex",
	"usr.chgSign",
	"usr.clearVipService",
	"usr.getSalary",
	"usr.heartTick",
	"usr.lazySync",
	"usr.recvSignRwd",
	"usr.refreshMedal",
	"usr.saveAuthInfo",
	"usr.saveCustIco",
	"usr.setMedalShow",
	"usr.share",
	"usr.triggerEvent",
	"usr.updateGuide",
	"usr.updateSoftGuide",
	"usr.updateUsrExt",
	"usr.updateUsrSet",
	"usr.updateVipService",
	"usr.upgrade",
	"usr.worship",
	"usrExtra.recvAntiFraudQARwd",
	"usrExtra.recvTtFansRwd",
	"usrExtra.recvVersionUpdateRwd",
	"usrExtra.recvWbFansRwd",
	"usrExtra.reportUsr",
	"usrExtra.setMsPwd",
	"usrExtra.setShowAddress",
	"usrExtra.shareMsg",
	"usrExtra.syncAddress",
	"usrExtra.updateAntiFraudQAStatus",
	"usrExtra.updateTaskMap",
	"usrExtra.updateTtSubscribe",
	"usrLand.clear",
	"usrLand.clearBatch",
	"usrLand.clearOneKey",
	"usrLand.harvest",
	"usrLand.harvestOneKey",
	"usrLand.plant",
	"usrLand.plantBatch",
	"usrLand.plantOneKey",
	"usrLand.refresh",
	"usrLand.speedUp",
	"usrLand.speedUpBatch",
	"usrLand.speedUpFree",
	"usrLand.speedUpOneKey",
	"usrLand.unlockLand",
	"usrLand.water",
	"usrLand.waterBatch",
	"usrLand.waterOneKey",
	"usrRed.delRed",
	"usrSubscribePush.addSubscribeNum",
	"usrSubscribePush.addSubscribeNumPermanent",
	"usrSubscribePush.msgPushSetting",
	"usrSubscribePush.msgPushSettingGlobal",
	"usrVerInfo.refresh",
	"valentines.apply",
	"valentines.bind",
	"valentines.enter",
	"valentines.frdStates",
	"valentines.recv",
	"valentines.reject",
	"valentines.unBind",
	"verify.checkVerification",
	"verify.refreshVerification",
	"vip.recv",
	"waterRqst.djst",
	"waterwheel.enter",
	"waterwheel.recv",
	"waterwheel.skip",
	"whiteDay26.apply",
	"whiteDay26.bind",
	"whiteDay26.enter",
	"whiteDay26.frdStates",
	"whiteDay26.recv",
	"whiteDay26.reject",
	"whiteDay26.unBind",
	"whiteValentine.apply",
	"whiteValentine.bind",
	"whiteValentine.enter",
	"whiteValentine.frdStates",
	"whiteValentine.recv",
	"whiteValentine.reject",
	"whiteValentine.unBind",
	"zoo.addFoodstuff",
	"zoo.calNaturalAtt",
	"zoo.changePetName",
	"zoo.enterZoo",
	"zoo.feedOtherPet",
	"zoo.feedPets",
	"zoo.findPet",
	"zoo.findPetByUsrBack",
	"zoo.getGuideEventRwd",
	"zoo.handBeOverdueEvent",
	"zoo.handleEvent",
	"zoo.initZoo",
	"zoo.readLog",
	"zoo.readSouvenir",
	"zoo.recvSouvenirRwd",
	"zoo.refreshPetStatus",
	"zoo.setUpSleepTime",
	"zoo.strokePet",
	"zoo.usePetItem",
	"zoo.visitZoo",
	"zooDecorate.equip",
	"zooDecorate.read",
}

var gameJSRPCNameSet = func() map[RPCName]struct{} {
	out := make(map[RPCName]struct{}, len(gameJSRPCNames))
	for _, name := range gameJSRPCNames {
		out[name] = struct{}{}
	}
	return out
}()

var gameJSRPCSpecs = []RPCSpec{
	{Name: RPCPlantRqstZhtc, Group: "PlantRqst", Method: "zhtc", RequestShape: RPCRequestFields, RequestFields: []string{"point"}, ResponseSchema: "StateDelta"},
	{Name: RPCReapPopupShjm, Group: "ReapPopup", Method: "shjm", RequestShape: RPCRequestFields, RequestFields: []string{"point"}, ResponseSchema: "StateDelta"},
	{Name: RPCActBuy, Group: "act", Method: "buy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "shopIdx", "shopItemId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGetOneOrderAward, Group: "act", Method: "getOneOrderAward", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "id", "lvl"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGetOrderAward, Group: "act", Method: "getOrderAward", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGetRankAward, Group: "act", Method: "getRankAward", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCActGetStat, Group: "act", Method: "getStat", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGiftBuy, Group: "act", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActRecv, Group: "act", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskIdx", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActRecvBoxes, Group: "act", Method: "recvBoxes", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskId", "wayType"}, ResponseSchema: "StateDelta"},
	{Name: RPCActRecvTLAward, Group: "act", Method: "recvTLAward", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActRefreshDailyGift, Group: "act", Method: "refreshDailyGift", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActRefreshTask, Group: "act", Method: "refreshTask", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSyncBatchInfo, Group: "act", Method: "syncBatchInfo", RequestShape: RPCRequestFields, RequestFields: []string{"batchIdList"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCallBackActCallBackBind, Group: "actCallBack", Method: "actCallBackBind", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCActCallBackActCallBackEnter, Group: "actCallBack", Method: "actCallBackEnter", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCActCallBackActCallBackRecv, Group: "actCallBack", Method: "actCallBackRecv", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectCheckLuckyCardSend, Group: "actCardCollect", Method: "checkLuckyCardSend", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectDeckShopExchange, Group: "actCardCollect", Method: "deckShopExchange", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "costStar"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectNextRound, Group: "actCardCollect", Method: "nextRound", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectOpenCardPack, Group: "actCardCollect", Method: "openCardPack", RequestShape: RPCRequestFields, RequestFields: []string{"cardPackId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectRecvBoxReward, Group: "actCardCollect", Method: "recvBoxReward", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "boxId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectRecvCardAlbumReward, Group: "actCardCollect", Method: "recvCardAlbumReward", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectRecvCollectReward, Group: "actCardCollect", Method: "recvCollectReward", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectRecvTaskReward, Group: "actCardCollect", Method: "recvTaskReward", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskIdx", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectRefreshTaskData, Group: "actCardCollect", Method: "refreshTaskData", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCardCollectUseSelectedCard, Group: "actCardCollect", Method: "useSelectedCard", RequestShape: RPCRequestFields, RequestFields: []string{"cardId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicNoteDirectRecvTaskRwd, Group: "actCyclicNote", Method: "directRecvTaskRwd", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicNoteGiftBuy, Group: "actCyclicNote", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicNoteReRandomTask, Group: "actCyclicNote", Method: "reRandomTask", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicNoteRecv, Group: "actCyclicNote", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicNoteRecvTaskRwd, Group: "actCyclicNote", Method: "recvTaskRwd", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicNoteResetGiftCd, Group: "actCyclicNote", Method: "resetGiftCd", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicNoteUnlockTaskSlot, Group: "actCyclicNote", Method: "unlockTaskSlot", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "slotId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicStoryGiftBuy, Group: "actCyclicStory", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicStoryReRandomOrder, Group: "actCyclicStory", Method: "reRandomOrder", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "orderIdx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicStoryRecv, Group: "actCyclicStory", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicStoryRecvOrderRwd, Group: "actCyclicStory", Method: "recvOrderRwd", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "orderIdx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicStoryRemoveOrderCd, Group: "actCyclicStory", Method: "removeOrderCd", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "orderIdx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicStoryResetGiftCd, Group: "actCyclicStory", Method: "resetGiftCd", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicVaseGiftBuy, Group: "actCyclicVase", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicVaseRecv, Group: "actCyclicVase", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActCyclicVaseResetGiftCd, Group: "actCyclicVase", Method: "resetGiftCd", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDessertEnter, Group: "actDessert", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDessertGameOver, Group: "actDessert", Method: "gameOver", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "gameType"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDessertGameStart, Group: "actDessert", Method: "gameStart", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDessertGameSync, Group: "actDessert", Method: "gameSync", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "gameType", "args"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDessertGiftBuy, Group: "actDessert", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDessertOpenBox, Group: "actDessert", Method: "openBox", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawDraw, Group: "actDraw", Method: "draw", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawChristmasDraw, Group: "actDrawChristmas", Method: "draw", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawChristmasEnter, Group: "actDrawChristmas", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawChristmasGiftBuy, Group: "actDrawChristmas", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawDragonDraw, Group: "actDrawDragon", Method: "draw", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawDragonGiftBuy, Group: "actDrawDragon", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawDragonRecv, Group: "actDrawDragon", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawGiftGiftBuy, Group: "actDrawGift", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawSprSkinDraw, Group: "actDrawSprSkin", Method: "draw", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawSprSkinEnter, Group: "actDrawSprSkin", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawSprSkinGiftBuy, Group: "actDrawSprSkin", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawZbDraw, Group: "actDrawZb", Method: "draw", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawZbEnter, Group: "actDrawZb", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActDrawZbGiftBuy, Group: "actDrawZb", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActElimEnter, Group: "actElim", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActElimGiftBuy, Group: "actElim", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActElimMove, Group: "actElim", Method: "move", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "model", "rowBefore", "colBefore", "rowAfter", "colAfter"}, ResponseSchema: "StateDelta"},
	{Name: RPCActElimOpenBox, Group: "actElim", Method: "openBox", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCActElimRefreshMap, Group: "actElim", Method: "refreshMap", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "model"}, ResponseSchema: "StateDelta"},
	{Name: RPCActElimUseItem1, Group: "actElim", Method: "useItem1", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "model"}, ResponseSchema: "StateDelta"},
	{Name: RPCActElimUseItem2, Group: "actElim", Method: "useItem2", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "model", "row", "col"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFlowerBattleChooseFlowerArt, Group: "actFlowerBattle", Method: "chooseFlowerArt", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "flowerArtId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFlowerBattleEnter, Group: "actFlowerBattle", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "isRefresh", "isCrossDay"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFlowerBattleGetGiftBuyRecords, Group: "actFlowerBattle", Method: "getGiftBuyRecords", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFlowerBattleGiftBuy, Group: "actFlowerBattle", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFlowerBattleLike, Group: "actFlowerBattle", Method: "like", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFlowerBattleRecvBoxesPrize, Group: "actFlowerBattle", Method: "recvBoxesPrize", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFlowerBattleSetIsAnonymous, Group: "actFlowerBattle", Method: "setIsAnonymous", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "isAnonymous"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFmlRedEnvelopeEnter, Group: "actFmlRedEnvelope", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFmlRedEnvelopeGetDetail, Group: "actFmlRedEnvelope", Method: "getDetail", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "id"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFmlRedEnvelopeGetRecord, Group: "actFmlRedEnvelope", Method: "getRecord", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFmlRedEnvelopeList, Group: "actFmlRedEnvelope", Method: "list", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFmlRedEnvelopePick, Group: "actFmlRedEnvelope", Method: "pick", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "id"}, ResponseSchema: "StateDelta"},
	{Name: RPCActFmlRedEnvelopeSend, Group: "actFmlRedEnvelope", Method: "send", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "itemId", "count", "msg"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGame2048Enter, Group: "actGame2048", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGame2048GiftBuy, Group: "actGame2048", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGame2048Move, Group: "actGame2048", Method: "move", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "map", "model", "dir"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGame2048OpenBox, Group: "actGame2048", Method: "openBox", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGame2048Restart, Group: "actGame2048", Method: "restart", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "map"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGame2048UseChange, Group: "actGame2048", Method: "useChange", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "map", "cells"}, ResponseSchema: "StateDelta"},
	{Name: RPCActGame2048UseEliminate, Group: "actGame2048", Method: "useEliminate", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "map", "cells"}, ResponseSchema: "StateDelta"},
	{Name: RPCActHoneyGiftBuy, Group: "actHoney", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActHoneyRecv, Group: "actHoney", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActHoneyResetGiftCd, Group: "actHoney", Method: "resetGiftCd", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActIPDmdGiftGiftBuy, Group: "actIPDmdGift", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActIPFlowerGuardOpenBox, Group: "actIPFlowerGuard", Method: "openBox", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "boxId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2Enter, Group: "actMerge2", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2Move, Group: "actMerge2", Method: "move", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "oRc", "tRc"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2OpenBox, Group: "actMerge2", Method: "openBox", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2PutInWarehouse, Group: "actMerge2", Method: "putInWarehouse", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "cell"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2PutOutTemp, Group: "actMerge2", Method: "putOutTemp", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2PutOutWarehouse, Group: "actMerge2", Method: "putOutWarehouse", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2RecvOrder, Group: "actMerge2", Method: "recvOrder", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "orderId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2RecvProgress, Group: "actMerge2", Method: "recvProgress", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2RefreshOrder, Group: "actMerge2", Method: "refreshOrder", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2SaveGuide, Group: "actMerge2", Method: "saveGuide", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "guideId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2SellItem, Group: "actMerge2", Method: "sellItem", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "rc"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2SplitItem, Group: "actMerge2", Method: "splitItem", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "sRc", "tRc"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2SwitchMode, Group: "actMerge2", Method: "switchMode", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "modeType"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2UnlockWarehouse, Group: "actMerge2", Method: "unlockWarehouse", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActMerge2UseItem, Group: "actMerge2", Method: "useItem", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "rc"}, ResponseSchema: "StateDelta"},
	{Name: RPCActOfficialsBuyItem, Group: "actOfficials", Method: "buyItem", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "itemId", "buyNum"}, ResponseSchema: "StateDelta"},
	{Name: RPCActOfficialsEnter, Group: "actOfficials", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActOfficialsRecvGrpReachPrize, Group: "actOfficials", Method: "recvGrpReachPrize", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActOfficialsUseItem, Group: "actOfficials", Method: "useItem", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "itemId", "useNum"}, ResponseSchema: "StateDelta"},
	{Name: RPCActPaperEnter, Group: "actPaper", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActPaperRecv, Group: "actPaper", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCActPaperRecvGamePrize, Group: "actPaper", Method: "recvGamePrize", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActPaperRecvTaskPrize, Group: "actPaper", Method: "recvTaskPrize", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActRchgRwdEnter, Group: "actRchgRwd", Method: "enter", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCActRchgRwdRecv, Group: "actRchgRwd", Method: "recv", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCActRchgWheelEnter, Group: "actRchgWheel", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActRchgWheelGetMyLog, Group: "actRchgWheel", Method: "getMyLog", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "index", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCActRchgWheelStartWheel, Group: "actRchgWheel", Method: "startWheel", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "drawNum"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSpoolEnter, Group: "actSpool", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "isInPage"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSpoolGameOver, Group: "actSpool", Method: "gameOver", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "gameType"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSpoolGameStart, Group: "actSpool", Method: "gameStart", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSpoolGameSync, Group: "actSpool", Method: "gameSync", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "gameType", "args"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSpoolGiftBuy, Group: "actSpool", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSpoolOpenBox, Group: "actSpool", Method: "openBox", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSpoolRise, Group: "actSpool", Method: "rise", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "gameType"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSpoolSetGuideStatus, Group: "actSpool", Method: "setGuideStatus", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActSpringTotRchgRecvTLAward, Group: "actSpringTotRchg", Method: "recvTLAward", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActVipTimeShopGiftBuy, Group: "actVipTimeShop", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCActZFBForestBrowseWeb, Group: "actZFBForest", Method: "browseWeb", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCActZFBForestBrowseWeb2, Group: "actZFBForest", Method: "browseWeb2", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCBagCombine, Group: "bag", Method: "combine", RequestShape: RPCRequestFields, RequestFields: []string{"iid", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCBagSell, Group: "bag", Method: "sell", RequestShape: RPCRequestFields, RequestFields: []string{"iidMap"}, ResponseSchema: "StateDelta"},
	{Name: RPCBagUse, Group: "bag", Method: "use", RequestShape: RPCRequestFields, RequestFields: []string{"iid", "num", "useDstValue"}, ResponseSchema: "StateDelta"},
	{Name: RPCBattlePassBuyLvl, Group: "battlePass", Method: "buyLvl", RequestShape: RPCRequestFields, RequestFields: []string{"bid", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCBattlePassRecv, Group: "battlePass", Method: "recv", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCBattlePassRecvAll, Group: "battlePass", Method: "recvAll", RequestShape: RPCRequestFields, RequestFields: []string{"bid"}, ResponseSchema: "StateDelta"},
	{Name: RPCBattlePassTaskDone, Group: "battlePass", Method: "taskDone", RequestShape: RPCRequestFields, RequestFields: []string{"bid", "taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCBenefitBoxDraw, Group: "benefitBox", Method: "draw", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCBestieApply, Group: "bestie", Method: "apply", RequestShape: RPCRequestFields, RequestFields: []string{"targetUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCBestieCancelDissolve, Group: "bestie", Method: "cancelDissolve", RequestShape: RPCRequestFields, RequestFields: []string{"bestieUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCBestieCheckApply, Group: "bestie", Method: "checkApply", RequestShape: RPCRequestFields, RequestFields: []string{"targetUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCBestieDissolve, Group: "bestie", Method: "dissolve", RequestShape: RPCRequestFields, RequestFields: []string{"bestieUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCBestieEnter, Group: "bestie", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCBestieGetFrdBestieCntMap, Group: "bestie", Method: "getFrdBestieCntMap", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCBestieHandleApply, Group: "bestie", Method: "handleApply", RequestShape: RPCRequestFields, RequestFields: []string{"applyUid", "accept"}, ResponseSchema: "StateDelta"},
	{Name: RPCBestieImmediateDissolve, Group: "bestie", Method: "immediateDissolve", RequestShape: RPCRequestFields, RequestFields: []string{"bestieUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCBestieSetSceneSkin, Group: "bestie", Method: "setSceneSkin", RequestShape: RPCRequestFields, RequestFields: []string{"bestieUid", "skinId"}, ResponseSchema: "StateDelta"},
	{Name: RPCBestieUnlockSlot, Group: "bestie", Method: "unlockSlot", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCBoostRecvRwd, Group: "boost", Method: "recvRwd", RequestShape: RPCRequestFields, RequestFields: []string{"type", "idx", "boxId"}, ResponseSchema: "StateDelta"},
	{Name: RPCBoostRefresh, Group: "boost", Method: "refresh", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCBubbleActiveBubble, Group: "bubble", Method: "activeBubble", RequestShape: RPCRequestFields, RequestFields: []string{"bubbleId"}, ResponseSchema: "StateDelta"},
	{Name: RPCBubbleChgBubble, Group: "bubble", Method: "chgBubble", RequestShape: RPCRequestFields, RequestFields: []string{"bubbleId"}, ResponseSchema: "StateDelta"},
	{Name: RPCCallFriendEnter, Group: "callFriend", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCCallFriendRecv, Group: "callFriend", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"id"}, ResponseSchema: "StateDelta"},
	{Name: RPCCallFriendUseCode, Group: "callFriend", Method: "useCode", RequestShape: RPCRequestFields, RequestFields: []string{"code"}, ResponseSchema: "StateDelta"},
	{Name: RPCCelebrityGetAllTypes, Group: "celebrity", Method: "getAllTypes", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCCelebrityGetAllTypesInfo, Group: "celebrity", Method: "getAllTypesInfo", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCCelebrityGetInfoByType, Group: "celebrity", Method: "getInfoByType", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCCelebrityLikeCelebrity, Group: "celebrity", Method: "likeCelebrity", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCChannelRwdRecvDailyDesktopRwd, Group: "channelRwd", Method: "recvDailyDesktopRwd", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCChannelRwdRecvFstDesktopRwd, Group: "channelRwd", Method: "recvFstDesktopRwd", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCChannelRwdRecvFstSidebarRwd, Group: "channelRwd", Method: "recvFstSidebarRwd", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCChannelRwdRecvLoginRwd, Group: "channelRwd", Method: "recvLoginRwd", RequestShape: RPCRequestFields, RequestFields: []string{"day"}, ResponseSchema: "StateDelta"},
	{Name: RPCCheaterDoCheat, Group: "cheater", Method: "doCheat", RequestShape: RPCRequestFields, RequestFields: []string{"sl"}, ResponseSchema: "StateDelta"},
	{Name: RPCCollectRwdRecv, Group: "collectRwd", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCCollectRwdRecvArtCreateRwd, Group: "collectRwd", Method: "recvArtCreateRwd", RequestShape: RPCRequestFields, RequestFields: []string{"flowerArtId"}, ResponseSchema: "StateDelta"},
	{Name: RPCCollectRwdRecvArtCreateByVase, Group: "collectRwd", Method: "recvArtCreateRwdByVase", RequestShape: RPCRequestFields, RequestFields: []string{"flowerArtId"}, ResponseSchema: "StateDelta"},
	{Name: RPCCultivateChooseSkill, Group: "cultivate", Method: "chooseSkill", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId", "slotId", "chooseType"}, ResponseSchema: "StateDelta"},
	{Name: RPCCultivateClearCulCD, Group: "cultivate", Method: "clearCulCd", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId"}, ResponseSchema: "StateDelta"},
	{Name: RPCCultivateCultivate, Group: "cultivate", Method: "cultivate", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId"}, ResponseSchema: "StateDelta"},
	{Name: RPCCultivateRandomSkill, Group: "cultivate", Method: "randomSkill", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId", "slotId"}, ResponseSchema: "StateDelta"},
	{Name: RPCCultivateRecv, Group: "cultivate", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId"}, ResponseSchema: "StateDelta"},
	{Name: RPCCultivateReduceByHelp, Group: "cultivate", Method: "reduceByHelp", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId", "helpUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCCultivateReduceByItem, Group: "cultivate", Method: "reduceByItem", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCCultivateUnlockSlot, Group: "cultivate", Method: "unlockSlot", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId", "slotId"}, ResponseSchema: "StateDelta"},
	{Name: RPCCultivateUpgrade, Group: "cultivate", Method: "upgrade", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId"}, ResponseSchema: "StateDelta"},
	{Name: RPCCustomerOrderRqstDkgkck, Group: "customerOrderRqst", Method: "dkgkck", RequestShape: RPCRequestFields, RequestFields: []string{"point"}, ResponseSchema: "StateDelta"},
	{Name: RPCDecorateBuild, Group: "decorate", Method: "build", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCDecorateBuildSuccess, Group: "decorate", Method: "buildSuccess", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCDecorateClearBuildCd, Group: "decorate", Method: "clearBuildCd", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCDecorateEquip, Group: "decorate", Method: "equip", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCDecorateRecv, Group: "decorate", Method: "recv", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCDecorateUpdateReadLvlList, Group: "decorate", Method: "updateReadLvlList", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCDrawDraw, Group: "draw", Method: "draw", RequestShape: RPCRequestFields, RequestFields: []string{"num"}, ResponseSchema: "StateDelta"},
	{Name: RPCDrawTestDrawVirtual, Group: "draw", Method: "testDrawVirtual", RequestShape: RPCRequestFields, RequestFields: []string{"num"}, ResponseSchema: "StateDelta"},
	{Name: RPCFashionEquip, Group: "fashion", Method: "equip", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFashionRead, Group: "fashion", Method: "read", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerArtMakeFlowerArt, Group: "flowerArt", Method: "makeFlowerArt", RequestShape: RPCRequestFields, RequestFields: []string{"vaseId", "flowersIds", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesCheckConvert, Group: "flowerElves", Method: "checkConvert", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesAidHelpFrd, Group: "flowerElvesAid", Method: "helpFrd", RequestShape: RPCRequestFields, RequestFields: []string{"dstUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesAidRecvAidEff, Group: "flowerElvesAid", Method: "recvAidEff", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesAidReqAid, Group: "flowerElvesAid", Method: "reqAid", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesBookUpgrade, Group: "flowerElvesBook", Method: "upgrade", RequestShape: RPCRequestFields, RequestFields: []string{"bookId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesBookDrawDraw, Group: "flowerElvesBookDraw", Method: "draw", RequestShape: RPCRequestFields, RequestFields: []string{"periodId", "gridPos"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesBookDrawRefresh, Group: "flowerElvesBookDraw", Method: "refresh", RequestShape: RPCRequestFields, RequestFields: []string{"periodId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesPlaceDispatch, Group: "flowerElvesPlace", Method: "dispatch", RequestShape: RPCRequestFields, RequestFields: []string{"placeId", "elvesId", "elvesNum", "iid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesPlaceRecv, Group: "flowerElvesPlace", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"placeId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesPlaceRecvAllReward, Group: "flowerElvesPlace", Method: "recvAllReward", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesPlaceSpeedUp, Group: "flowerElvesPlace", Method: "speedUp", RequestShape: RPCRequestFields, RequestFields: []string{"placeId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerElvesPlaceUnlock, Group: "flowerElvesPlace", Method: "unlock", RequestShape: RPCRequestFields, RequestFields: []string{"placeId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerGiftRecvBox, Group: "flowerGift", Method: "recvBox", RequestShape: RPCRequestFields, RequestFields: []string{"pageId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketBuyFlower, Group: "flowerMarket", Method: "buyFlower", RequestShape: RPCRequestFields, RequestFields: []string{"sellerUid", "shelfId", "flower", "password"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketBuyPutCount, Group: "flowerMarket", Method: "buyPutCount", RequestShape: RPCRequestFields, RequestFields: []string{"count"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketCheckPassword, Group: "flowerMarket", Method: "checkPassword", RequestShape: RPCRequestFields, RequestFields: []string{"sellerUid", "shelfId", "password"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketEnter, Group: "flowerMarket", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketGetFriend, Group: "flowerMarket", Method: "getFriend", RequestShape: RPCRequestFields, RequestFields: []string{"frdUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketGetFriendList, Group: "flowerMarket", Method: "getFriendList", RequestShape: RPCRequestFields, RequestFields: []string{"specificFriendIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketGetTradeRecords, Group: "flowerMarket", Method: "getTradeRecords", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketHarvestIncome, Group: "flowerMarket", Method: "harvestIncome", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketPutFlower, Group: "flowerMarket", Method: "putFlower", RequestShape: RPCRequestFields, RequestFields: []string{"shelfId", "flowerId", "count", "priceIdx", "password"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketPutFlowerBatch, Group: "flowerMarket", Method: "putFlowerBatch", RequestShape: RPCRequestFields, RequestFields: []string{"shelfIds", "flowerId", "count", "priceIdx", "password"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketTakeDownFlower, Group: "flowerMarket", Method: "takeDownFlower", RequestShape: RPCRequestFields, RequestFields: []string{"shelfId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerMarketUnlockShelf, Group: "flowerMarket", Method: "unlockShelf", RequestShape: RPCRequestFields, RequestFields: []string{"shelfId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerOrderRqstShowR, Group: "flowerOrderRqst", Method: "showR", RequestShape: RPCRequestFields, RequestFields: []string{"point"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerRackCancelSell, Group: "flowerRack", Method: "cancelSell", RequestShape: RPCRequestFields, RequestFields: []string{"rackId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerRackRecvOneKey, Group: "flowerRack", Method: "recvOneKey", RequestShape: RPCRequestFields, RequestFields: []string{"standId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerRackRecvSellMoney, Group: "flowerRack", Method: "recvSellMoney", RequestShape: RPCRequestFields, RequestFields: []string{"rackId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerRackSell, Group: "flowerRack", Method: "sell", RequestShape: RPCRequestFields, RequestFields: []string{"rackId", "iid", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCFlowerRackUnlockStand, Group: "flowerRack", Method: "unlockStand", RequestShape: RPCRequestFields, RequestFields: []string{"standId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlAutoJoin, Group: "fml", Method: "autoJoin", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlBld, Group: "fml", Method: "bld", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlBuyRaceBoat, Group: "fml", Method: "buyRaceBoat", RequestShape: RPCRequestFields, RequestFields: []string{"boatId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlChgPos, Group: "fml", Method: "chgPos", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlChgTitle, Group: "fml", Method: "chgTitle", RequestShape: RPCRequestFields, RequestFields: []string{"titleId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlClearQuitTime, Group: "fml", Method: "clearQuitTime", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlCreate, Group: "fml", Method: "create", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlDissolve, Group: "fml", Method: "dissolve", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlEnter, Group: "fml", Method: "enter", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlEquipRaceBoat, Group: "fml", Method: "equipRaceBoat", RequestShape: RPCRequestFields, RequestFields: []string{"boatId", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlGetHonor, Group: "fml", Method: "getHonor", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlGetLog, Group: "fml", Method: "getLog", RequestShape: RPCRequestFields, RequestFields: []string{"fid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlGetMedalRwd, Group: "fml", Method: "getMedalRwd", RequestShape: RPCRequestFields, RequestFields: []string{"medalId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlGetRecFmlList, Group: "fml", Method: "getRecFmlList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlGetTitleLogList, Group: "fml", Method: "getTitleLogList", RequestShape: RPCRequestFields, RequestFields: []string{"titleId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlHandleApply, Group: "fml", Method: "handleApply", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlHandleApplyAll, Group: "fml", Method: "handleApplyAll", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlHandleInv, Group: "fml", Method: "handleInv", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlInv, Group: "fml", Method: "inv", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlJoin, Group: "fml", Method: "join", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlKick, Group: "fml", Method: "kick", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlOpenFmlRaceBox, Group: "fml", Method: "openFmlRaceBox", RequestShape: RPCRequestFields, RequestFields: []string{"isAll"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlQuit, Group: "fml", Method: "quit", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRecvBox, Group: "fml", Method: "recvBox", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRefreshRaceBoat, Group: "fml", Method: "refreshRaceBoat", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRefreshTitle, Group: "fml", Method: "refreshTitle", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlSearch, Group: "fml", Method: "search", RequestShape: RPCRequestFields, RequestFields: []string{"fid", "withMb"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlSetting, Group: "fml", Method: "setting", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlUnbindUnionGroup, Group: "fml", Method: "unbindUnionGroup", RequestShape: RPCRequestFields, RequestFields: []string{"fmlId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlUnloadRaceBoat, Group: "fml", Method: "unloadRaceBoat", RequestShape: RPCRequestFields, RequestFields: []string{"boatId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlUpgradeFml, Group: "fml", Method: "upgradeFml", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlUpgradeRaceBoat, Group: "fml", Method: "upgradeRaceBoat", RequestShape: RPCRequestFields, RequestFields: []string{"boatId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShareAddTakeCnt, Group: "fmlFlowerShare", Method: "addTakeCnt", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShareGetFmlOtherShareList, Group: "fmlFlowerShare", Method: "getFmlOtherShareList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShareGetShareLogList, Group: "fmlFlowerShare", Method: "getShareLogList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShareRecvRwd, Group: "fmlFlowerShare", Method: "recvRwd", RequestShape: RPCRequestFields, RequestFields: []string{"slotIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShareRefresh, Group: "fmlFlowerShare", Method: "refresh", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShareShare, Group: "fmlFlowerShare", Method: "share", RequestShape: RPCRequestFields, RequestFields: []string{"slotId", "flowerId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShareTake, Group: "fmlFlowerShare", Method: "take", RequestShape: RPCRequestFields, RequestFields: []string{"dstUid", "slotId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShareUnlock, Group: "fmlFlowerShare", Method: "unlock", RequestShape: RPCRequestFields, RequestFields: []string{"slotId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShowCancelLikeOther, Group: "fmlFlowerShow", Method: "cancelLikeOther", RequestShape: RPCRequestFields, RequestFields: []string{"targetUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShowGetLikeOtherRecord, Group: "fmlFlowerShow", Method: "getLikeOtherRecord", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShowGetLikeOtherRecord5Limit, Group: "fmlFlowerShow", Method: "getLikeOtherRecord5Limit", RequestShape: RPCRequestFields, RequestFields: []string{"uid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShowGetShowInfo, Group: "fmlFlowerShow", Method: "getShowInfo", RequestShape: RPCRequestFields, RequestFields: []string{"targetUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShowLikeOther, Group: "fmlFlowerShow", Method: "likeOther", RequestShape: RPCRequestFields, RequestFields: []string{"targetUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShowSaveShow, Group: "fmlFlowerShow", Method: "saveShow", RequestShape: RPCRequestFields, RequestFields: []string{"showFlowers"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShowSetVisitType, Group: "fmlFlowerShow", Method: "setVisitType", RequestShape: RPCRequestFields, RequestFields: []string{"visitType"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShowSwitchMap, Group: "fmlFlowerShow", Method: "switchMap", RequestShape: RPCRequestFields, RequestFields: []string{"mapId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlFlowerShowUnlockSlot, Group: "fmlFlowerShow", Method: "unlockSlot", RequestShape: RPCRequestFields, RequestFields: []string{"index"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestApplyPlant, Group: "fmlForest", Method: "applyPlant", RequestShape: RPCRequestFields, RequestFields: []string{"treeId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestCollectEnergy, Group: "fmlForest", Method: "collectEnergy", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestEnter, Group: "fmlForest", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestGetCertDetail, Group: "fmlForest", Method: "getCertDetail", RequestShape: RPCRequestFields, RequestFields: []string{"treeCodeList"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestGetCertDetailByFid, Group: "fmlForest", Method: "getCertDetailByFid", RequestShape: RPCRequestFields, RequestFields: []string{"fid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestGetLogList, Group: "fmlForest", Method: "getLogList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestGetTreeList, Group: "fmlForest", Method: "getTreeList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestGetWeekCollect, Group: "fmlForest", Method: "getWeekCollect", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestGetWeekStat, Group: "fmlForest", Method: "getWeekStat", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlForestRefresh, Group: "fmlForest", Method: "refresh", RequestShape: RPCRequestFields, RequestFields: []string{"isAutoCollect"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlLandHarvest, Group: "fmlLand", Method: "harvest", RequestShape: RPCRequestFields, RequestFields: []string{"landIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlLandHarvestAll, Group: "fmlLand", Method: "harvestAll", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlLandPlant, Group: "fmlLand", Method: "plant", RequestShape: RPCRequestFields, RequestFields: []string{"landIds", "flwId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlLandUnlock, Group: "fmlLand", Method: "unlock", RequestShape: RPCRequestFields, RequestFields: []string{"landId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlLandUpgrade, Group: "fmlLand", Method: "upgrade", RequestShape: RPCRequestFields, RequestFields: []string{"landId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceBuyTaskNum, Group: "fmlRace", Method: "buyTaskNum", RequestShape: RPCRequestFields, RequestFields: []string{"num"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceDelTask, Group: "fmlRace", Method: "delTask", RequestShape: RPCRequestFields, RequestFields: []string{"taskMsId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceEnter, Group: "fmlRace", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceFinishTask, Group: "fmlRace", Method: "finishTask", RequestShape: RPCRequestFields, RequestFields: []string{"taskMsId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceGetFmlRaceEndDisplayData, Group: "fmlRace", Method: "getFmlRaceEndDisplayData", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceGetFmlRaceHistRcdList, Group: "fmlRace", Method: "getFmlRaceHistRcdList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceGetFmlRaceTaskScore, Group: "fmlRace", Method: "getFmlRaceTaskScore", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceGetFmlRaceUsrRankList, Group: "fmlRace", Method: "getFmlRaceUsrRankList", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceGetGroupFmlRaceRcdList, Group: "fmlRace", Method: "getGroupFmlRaceRcdList", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "groupId", "isRefresh"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceGetNewMbScoreRank, Group: "fmlRace", Method: "getNewMbScoreRank", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceGetTaskList, Group: "fmlRace", Method: "getTaskList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceGetTaskLogList, Group: "fmlRace", Method: "getTaskLogList", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceGiveUpTask, Group: "fmlRace", Method: "giveUpTask", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceRefreshFmlRaceBatch, Group: "fmlRace", Method: "refreshFmlRaceBatch", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceRefreshFmlRaceBox, Group: "fmlRace", Method: "refreshFmlRaceBox", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceRefreshTask, Group: "fmlRace", Method: "refreshTask", RequestShape: RPCRequestFields, RequestFields: []string{"idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceTakeTask, Group: "fmlRace", Method: "takeTask", RequestShape: RPCRequestFields, RequestFields: []string{"taskMsId"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceUpgradeTask, Group: "fmlRace", Method: "upgradeTask", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFmlRaceRqstShowShip, Group: "fmlRaceRqst", Method: "showShip", RequestShape: RPCRequestFields, RequestFields: []string{"time"}, ResponseSchema: "StateDelta"},
	{Name: RPCFmlTaskEnterShowtcrw, Group: "fmlTaskEnter", Method: "showtcrw", RequestShape: RPCRequestFields, RequestFields: []string{"point"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdAddBlack, Group: "frd", Method: "addBlack", RequestShape: RPCRequestFields, RequestFields: []string{"uid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdApplyFrd, Group: "frd", Method: "applyFrd", RequestShape: RPCRequestFields, RequestFields: []string{"uid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdApplyFrdBatch, Group: "frd", Method: "applyFrdBatch", RequestShape: RPCRequestFields, RequestFields: []string{"uids"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdDel, Group: "frd", Method: "del", RequestShape: RPCRequestFields, RequestFields: []string{"uid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdDelBlack, Group: "frd", Method: "delBlack", RequestShape: RPCRequestFields, RequestFields: []string{"uid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdEnter, Group: "frd", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"needBlackList", "needApplyList", "needFriendList"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdGetApplyList, Group: "frd", Method: "getApplyList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFrdGetBlackList, Group: "frd", Method: "getBlackList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFrdGetFriendList, Group: "frd", Method: "getFriendList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFrdHandleApply, Group: "frd", Method: "handleApply", RequestShape: RPCRequestFields, RequestFields: []string{"uid", "agree"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdHandleApplyBatch, Group: "frd", Method: "handleApplyBatch", RequestShape: RPCRequestFields, RequestFields: []string{"uids", "agree"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdLike, Group: "frd", Method: "like", RequestShape: RPCRequestFields, RequestFields: []string{"uid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdRefreshRecList, Group: "frd", Method: "refreshRecList", RequestShape: RPCRequestFields, RequestFields: []string{"isFree"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdSetFrdRjt, Group: "frd", Method: "setFrdRjt", RequestShape: RPCRequestFields, RequestFields: []string{"isRjtFrd"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdExtBuyStealCnt, Group: "frdExt", Method: "buyStealCnt", RequestShape: RPCRequestFields, RequestFields: []string{"frdUid", "buyCnt"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdExtCancelFollow, Group: "frdExt", Method: "cancelFollow", RequestShape: RPCRequestFields, RequestFields: []string{"frdUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdExtFollow, Group: "frdExt", Method: "follow", RequestShape: RPCRequestFields, RequestFields: []string{"frdUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdExtGetFrdOtherInfoByUids, Group: "frdExt", Method: "getFrdOtherInfoByUids", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFrdExtSearchUser, Group: "frdExt", Method: "searchUser", RequestShape: RPCRequestFields, RequestFields: []string{"keyword"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdHomeGetFrdHomeInfo, Group: "frdHome", Method: "getFrdHomeInfo", RequestShape: RPCRequestFields, RequestFields: []string{"frdUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdShareEnter, Group: "frdShare", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFrdShareRecvBoxRwd, Group: "frdShare", Method: "recvBoxRwd", RequestShape: RPCRequestFields, RequestFields: []string{"idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdShareRecvSelfRwd, Group: "frdShare", Method: "recvSelfRwd", RequestShape: RPCRequestFields, RequestFields: []string{"idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdShareRecvShareRwd, Group: "frdShare", Method: "recvShareRwd", RequestShape: RPCRequestFields, RequestFields: []string{"weekId", "inviterUid", "idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdStealGetFrdStealRcdList, Group: "frdSteal", Method: "getFrdStealRcdList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCFrdStealGetStealStateByUids, Group: "frdSteal", Method: "getStealStateByUids", RequestShape: RPCRequestFields, RequestFields: []string{"uids"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdStealSteal, Group: "frdSteal", Method: "steal", RequestShape: RPCRequestFields, RequestFields: []string{"frdUid", "landId", "stealElves"}, ResponseSchema: "StateDelta"},
	{Name: RPCFrdStealStealOneKey, Group: "frdSteal", Method: "stealOneKey", RequestShape: RPCRequestFields, RequestFields: []string{"frdUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCFreeWaterRecv, Group: "freeWater", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCGameClubEnter, Group: "gameClub", Method: "enter", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCGameClubEnterClub, Group: "gameClub", Method: "enterClub", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCGameClubRecvTaskRwd, Group: "gameClub", Method: "recvTaskRwd", RequestShape: RPCRequestFields, RequestFields: []string{"taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCGirlsDayApply, Group: "girlsDay", Method: "apply", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCGirlsDayBind, Group: "girlsDay", Method: "bind", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCGirlsDayEnter, Group: "girlsDay", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCGirlsDayFrdStates, Group: "girlsDay", Method: "frdStates", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "uids"}, ResponseSchema: "StateDelta"},
	{Name: RPCGirlsDayRecv, Group: "girlsDay", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCGirlsDayReject, Group: "girlsDay", Method: "reject", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCGirlsDayUnBind, Group: "girlsDay", Method: "unBind", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCGiveGiftGetGiveUidList, Group: "giveGift", Method: "getGiveUidList", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "giftId"}, ResponseSchema: "StateDelta"},
	{Name: RPCHomeRqstShowBird, Group: "homeRqst", Method: "showBird", RequestShape: RPCRequestFields, RequestFields: []string{"time"}, ResponseSchema: "StateDelta"},
	{Name: RPCIcoFrameActiveIcoFrame, Group: "icoFrame", Method: "activeIcoFrame", RequestShape: RPCRequestFields, RequestFields: []string{"frameId"}, ResponseSchema: "StateDelta"},
	{Name: RPCIcoFrameChgIcoFrame, Group: "icoFrame", Method: "chgIcoFrame", RequestShape: RPCRequestFields, RequestFields: []string{"frameId"}, ResponseSchema: "StateDelta"},
	{Name: RPCImChangeChannel, Group: "im", Method: "changeChannel", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCImDelChatPri, Group: "im", Method: "delChatPri", RequestShape: RPCRequestFields, RequestFields: []string{"toUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCImDelChatPriRed, Group: "im", Method: "delChatPriRed", RequestShape: RPCRequestFields, RequestFields: []string{"toUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCImEnter, Group: "im", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"roomId", "lastIdx", "lastEnterIdx", "missingRanges"}, ResponseSchema: "StateDelta"},
	{Name: RPCImGetChannelId, Group: "im", Method: "getChannelId", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCImRead, Group: "im", Method: "read", RequestShape: RPCRequestFields, RequestFields: []string{"msgId"}, ResponseSchema: "StateDelta"},
	{Name: RPCImRefuseStranger, Group: "im", Method: "refuseStranger", RequestShape: RPCRequestFields, RequestFields: []string{"isRefuse"}, ResponseSchema: "StateDelta"},
	{Name: RPCImSend, Group: "im", Method: "send", RequestShape: RPCRequestFields, RequestFields: []string{"roomId", "type", "content", "cms", "ext"}, ResponseSchema: "StateDelta"},
	{Name: RPCIndexCreateUsr, Group: "index", Method: "createUsr", RequestShape: RPCRequestFields, RequestFields: []string{"aid", "gsIdx", "token", "isNative", "nick", "sex", "ico", "ext", "inviter"}, ResponseSchema: "StateDelta"},
	{Name: RPCIndexLogin, Group: "index", Method: "login", RequestShape: RPCRequestFields, RequestFields: []string{"aid", "gsIdx", "token", "osType", "isNative", "deviceId", "isSimulator", "deviceInfo", "inviter", "shareExt", "version", "area", "chnId"}, ResponseSchema: "StateDelta"},
	{Name: RPCIndexReLogin, Group: "index", Method: "reLogin", RequestShape: RPCRequestFields, RequestFields: []string{"aid", "gsIdx", "token", "osType", "isNative", "deviceId", "isSimulator", "deviceInfo", "inviter", "shareExt", "version", "area", "chnId"}, ResponseSchema: "StateDelta"},
	{Name: RPCMailDel, Group: "mail", Method: "del", RequestShape: RPCRequestFields, RequestFields: []string{"msId", "allId"}, ResponseSchema: "StateDelta"},
	{Name: RPCMailDelOneKey, Group: "mail", Method: "delOneKey", RequestShape: RPCRequestFields, RequestFields: []string{"mode"}, ResponseSchema: "StateDelta"},
	{Name: RPCMailGetList, Group: "mail", Method: "getList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCMailOper, Group: "mail", Method: "oper", RequestShape: RPCRequestFields, RequestFields: []string{"msId", "allId"}, ResponseSchema: "StateDelta"},
	{Name: RPCMailPick, Group: "mail", Method: "pick", RequestShape: RPCRequestFields, RequestFields: []string{"msId", "allId"}, ResponseSchema: "StateDelta"},
	{Name: RPCMailPickOneKey, Group: "mail", Method: "pickOneKey", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCMailRead, Group: "mail", Method: "read", RequestShape: RPCRequestFields, RequestFields: []string{"msId", "allId"}, ResponseSchema: "StateDelta"},
	{Name: RPCMiniGameEndMiniGame, Group: "miniGame", Method: "endMiniGame", RequestShape: RPCRequestFields, RequestFields: []string{"copyId", "type"}, ResponseSchema: "StateDelta"},
	{Name: RPCMiniGameEnterMiniGame, Group: "miniGame", Method: "enterMiniGame", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCMiniGameStartMiniGame, Group: "miniGame", Method: "startMiniGame", RequestShape: RPCRequestFields, RequestFields: []string{"copyId", "type"}, ResponseSchema: "StateDelta"},
	{Name: RPCMiscBuyMonthCard, Group: "misc", Method: "buyMonthCard", RequestShape: RPCRequestFields, RequestFields: []string{"num"}, ResponseSchema: "StateDelta"},
	{Name: RPCMiscGetAdvanceWashItem, Group: "misc", Method: "getAdvanceWashItem", RequestShape: RPCRequestFields, RequestFields: []string{"itemId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCMiscRecvMsgPushRwd, Group: "misc", Method: "recvMsgPushRwd", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCMiscReportCheckBw, Group: "misc", Method: "reportCheckBw", RequestShape: RPCRequestFields, RequestFields: []string{"dstUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCMiscSellFlower, Group: "misc", Method: "sellFlower", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCMiscSyncItemHide, Group: "misc", Method: "syncItemHide", RequestShape: RPCRequestFields, RequestFields: []string{"version", "itemIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCMonthFlowerBuy, Group: "monthFlower", Method: "buy", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCMonthFlowerEnter, Group: "monthFlower", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOpptGetDetailOppts, Group: "oppt", Method: "getDetailOppts", RequestShape: RPCRequestFields, RequestFields: []string{"uids", "extKeys"}, ResponseSchema: "StateDelta"},
	{Name: RPCOpptGetOppt, Group: "oppt", Method: "getOppt", RequestShape: RPCRequestFields, RequestFields: []string{"uid"}, ResponseSchema: "StateDelta"},
	{Name: RPCOpptGetOppts, Group: "oppt", Method: "getOppts", RequestShape: RPCRequestFields, RequestFields: []string{"uids", "force"}, ResponseSchema: "StateDelta"},
	{Name: RPCOrderCustomerFinishOrder, Group: "orderCustomer", Method: "finishOrder", RequestShape: RPCRequestFields, RequestFields: []string{"npcId"}, ResponseSchema: "StateDelta"},
	{Name: RPCOrderCustomerGenOrder, Group: "orderCustomer", Method: "genOrder", RequestShape: RPCRequestFields, RequestFields: []string{"guestNpcIdList"}, ResponseSchema: "StateDelta"},
	{Name: RPCOrderCustomerRejectOrder, Group: "orderCustomer", Method: "rejectOrder", RequestShape: RPCRequestFields, RequestFields: []string{"npcId"}, ResponseSchema: "StateDelta"},
	{Name: RPCOrderFlowerEnter, Group: "orderFlower", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderFlowerFinishDecorateOrder, Group: "orderFlower", Method: "finishDecorateOrder", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderFlowerFinishOrder, Group: "orderFlower", Method: "finishOrder", RequestShape: RPCRequestFields, RequestFields: []string{"boxId"}, ResponseSchema: "StateDelta"},
	{Name: RPCOrderFlowerFinishSatinOrder, Group: "orderFlower", Method: "finishSatinOrder", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderFlowerRecvOrderRwd, Group: "orderFlower", Method: "recvOrderRwd", RequestShape: RPCRequestFields, RequestFields: []string{"target"}, ResponseSchema: "StateDelta"},
	{Name: RPCOrderFlowerRessuieOrderRwd, Group: "orderFlower", Method: "ressuieOrderRwd", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderPalaceEnter, Group: "orderPalace", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderPalaceFinishOrder, Group: "orderPalace", Method: "finishOrder", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderPalaceGetOrderRcdList, Group: "orderPalace", Method: "getOrderRcdList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderPalaceRefreshOrder, Group: "orderPalace", Method: "refreshOrder", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderTeamRecvRwd, Group: "orderTeam", Method: "recvRwd", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderTeamRefreshOrder, Group: "orderTeam", Method: "refreshOrder", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderTeamStoreOrder, Group: "orderTeam", Method: "storeOrder", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderTeamSubmitOrder, Group: "orderTeam", Method: "submitOrder", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCOrderTeamTakeOrder, Group: "orderTeam", Method: "takeOrder", RequestShape: RPCRequestFields, RequestFields: []string{"isAgree", "isCost"}, ResponseSchema: "StateDelta"},
	{Name: RPCOrderTeamTakeStoredOrder, Group: "orderTeam", Method: "takeStoredOrder", RequestShape: RPCRequestFields, RequestFields: []string{"npcId"}, ResponseSchema: "StateDelta"},
	{Name: RPCPearlDraw, Group: "pearl", Method: "draw", RequestShape: RPCRequestFields, RequestFields: []string{"count"}, ResponseSchema: "StateDelta"},
	{Name: RPCPearlGetHireMyLog, Group: "pearl", Method: "getHireMyLog", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPearlGetHireStateByUids, Group: "pearl", Method: "getHireStateByUids", RequestShape: RPCRequestFields, RequestFields: []string{"uids"}, ResponseSchema: "StateDelta"},
	{Name: RPCPearlGetMyHireLog, Group: "pearl", Method: "getMyHireLog", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPearlGetRecommendList, Group: "pearl", Method: "getRecommendList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPearlRecvDailyFree, Group: "pearl", Method: "recvDailyFree", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPearlRefresh, Group: "pearl", Method: "refresh", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPearlSetProtectState, Group: "pearl", Method: "setProtectState", RequestShape: RPCRequestFields, RequestFields: []string{"protectState"}, ResponseSchema: "StateDelta"},
	{Name: RPCPearlPlaceHire, Group: "pearlPlace", Method: "hire", RequestShape: RPCRequestFields, RequestFields: []string{"placeId", "dstUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCPearlPlaceRecv, Group: "pearlPlace", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"placeId"}, ResponseSchema: "StateDelta"},
	{Name: RPCPearlPlaceRecvOneKey, Group: "pearlPlace", Method: "recvOneKey", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPearlPlaceUnlockPlace, Group: "pearlPlace", Method: "unlockPlace", RequestShape: RPCRequestFields, RequestFields: []string{"placeId"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoBuy, Group: "photo", Method: "buy", RequestShape: RPCRequestFields, RequestFields: []string{"tempId"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoBuyTicket, Group: "photo", Method: "buyTicket", RequestShape: RPCRequestFields, RequestFields: []string{"num"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoCheckInvite, Group: "photo", Method: "checkInvite", RequestShape: RPCRequestFields, RequestFields: []string{"inviteId"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoCloseRoom, Group: "photo", Method: "closeRoom", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoDelRoomUsr, Group: "photo", Method: "delRoomUsr", RequestShape: RPCRequestFields, RequestFields: []string{"type", "dstUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoEnter, Group: "photo", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoEnterRoom, Group: "photo", Method: "enterRoom", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoFinishRoom, Group: "photo", Method: "finishRoom", RequestShape: RPCRequestFields, RequestFields: []string{"type", "info"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoGetBase64, Group: "photo", Method: "getBase64", RequestShape: RPCRequestFields, RequestFields: []string{"list"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoGetFriendList, Group: "photo", Method: "getFriendList", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoGetInviteList, Group: "photo", Method: "getInviteList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoGetPhotoList, Group: "photo", Method: "getPhotoList", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoInvite, Group: "photo", Method: "invite", RequestShape: RPCRequestFields, RequestFields: []string{"type", "dstUids"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoInviteDeal, Group: "photo", Method: "inviteDeal", RequestShape: RPCRequestFields, RequestFields: []string{"inviteId", "info"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoPushBase64, Group: "photo", Method: "pushBase64", RequestShape: RPCRequestFields, RequestFields: []string{"plainText", "md5", "idx", "maxIdx", "usePNG"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoReadInvite, Group: "photo", Method: "readInvite", RequestShape: RPCRequestFields, RequestFields: []string{"inviteIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoReadPhoto, Group: "photo", Method: "readPhoto", RequestShape: RPCRequestFields, RequestFields: []string{"md5List"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoReadRoomMsg, Group: "photo", Method: "readRoomMsg", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoReclaimRoom, Group: "photo", Method: "reclaimRoom", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoRejectInvite, Group: "photo", Method: "rejectInvite", RequestShape: RPCRequestFields, RequestFields: []string{"dstUid", "roomId"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoReshoot, Group: "photo", Method: "reshoot", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoSavePhoto, Group: "photo", Method: "savePhoto", RequestShape: RPCRequestFields, RequestFields: []string{"inviteId"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoSaveRoomPhoto, Group: "photo", Method: "saveRoomPhoto", RequestShape: RPCRequestFields, RequestFields: []string{"type", "isSave"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoSaveRoomPro, Group: "photo", Method: "saveRoomPro", RequestShape: RPCRequestFields, RequestFields: []string{"type", "progress"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoSaveRoomUsr, Group: "photo", Method: "saveRoomUsr", RequestShape: RPCRequestFields, RequestFields: []string{"type", "info"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoSetPhotoStatus, Group: "photo", Method: "setPhotoStatus", RequestShape: RPCRequestFields, RequestFields: []string{"md5List", "status"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoSetRefuseInvite, Group: "photo", Method: "setRefuseInvite", RequestShape: RPCRequestFields, RequestFields: []string{"isRefuse"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoTakePhoto, Group: "photo", Method: "takePhoto", RequestShape: RPCRequestFields, RequestFields: []string{"type", "md5"}, ResponseSchema: "StateDelta"},
	{Name: RPCPhotoTransmitRoom, Group: "photo", Method: "transmitRoom", RequestShape: RPCRequestFields, RequestFields: []string{"type", "operaUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCPlayerBackPlayerBackPassEnter, Group: "playerBack", Method: "playerBackPassEnter", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPlayerBackPlayerBackPassRecv, Group: "playerBack", Method: "playerBackPassRecv", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPlayerBackPlayerBackPassRecvOneKey, Group: "playerBack", Method: "playerBackPassRecvOneKey", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPlayerBackPlayerBackPassTaskDone, Group: "playerBack", Method: "playerBackPassTaskDone", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPlayerBackSign, Group: "playerBack", Method: "sign", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPlayerBackSignEnter, Group: "playerBack", Method: "signEnter", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPlayerBackSignRecv, Group: "playerBack", Method: "signRecv", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCPlayerBackUpdateGuildIds, Group: "playerBack", Method: "updateGuildIds", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCRandomEventDoAffair, Group: "randomEvent", Method: "doAffair", RequestShape: RPCRequestFields, RequestFields: []string{"eventId"}, ResponseSchema: "StateDelta"},
	{Name: RPCRandomEventEnter, Group: "randomEvent", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCRankGetRanks, Group: "rank", Method: "getRanks", RequestShape: RPCRequestFields, RequestFields: []string{"list", "masks"}, ResponseSchema: "StateDelta"},
	{Name: RPCRchgCardRecv, Group: "rchgCard", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCRchgDayEnter, Group: "rchgDay", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCRchgDayReceive, Group: "rchgDay", Method: "receive", RequestShape: RPCRequestFields, RequestFields: []string{"index"}, ResponseSchema: "StateDelta"},
	{Name: RPCRchgOrderToMoneyConvertMoney, Group: "rchgOrderToMoney", Method: "convertMoney", RequestShape: RPCRequestFields, RequestFields: []string{"orderNo"}, ResponseSchema: "StateDelta"},
	{Name: RPCRchgSumRecv, Group: "rchgSum", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"id"}, ResponseSchema: "StateDelta"},
	{Name: RPCRedeemGetInfo, Group: "redeem", Method: "getInfo", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCRedeemUseCode, Group: "redeem", Method: "useCode", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCRedeemCodeShowDjdk, Group: "redeemCodeShow", Method: "djdk", RequestShape: RPCRequestFields, RequestFields: []string{"point"}, ResponseSchema: "StateDelta"},
	{Name: RPCReputationAppeal, Group: "reputation", Method: "appeal", RequestShape: RPCRequestFields, RequestFields: []string{"reason", "msIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCReputationGetLogs, Group: "reputation", Method: "getLogs", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCReputationView, Group: "reputation", Method: "view", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCReserveCheckRwd, Group: "reserve", Method: "checkRwd", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCRoadGrowRecv, Group: "roadGrow", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"id"}, ResponseSchema: "StateDelta"},
	{Name: RPCRoadGrowRecvBox, Group: "roadGrow", Method: "recvBox", RequestShape: RPCRequestFields, RequestFields: []string{"idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCRwdRecv, Group: "rwd", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCRwdSetCanRecv, Group: "rwd", Method: "setCanRecv", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCSdkCheckRchg, Group: "sdk", Method: "checkRchg", RequestShape: RPCRequestFields, RequestFields: []string{"rchgId", "type", "value", "ext", "useMoney", "useMoneyCount", "requestFriendPayment"}, ResponseSchema: "StateDelta"},
	{Name: RPCSdkMoniPay, Group: "sdk", Method: "moniPay", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSdkPayByMoney, Group: "sdk", Method: "payByMoney", RequestShape: RPCRequestFields, RequestFields: []string{"id", "orderno", "sign", "rechargeType", "rechargeTypeValue", "subjectName", "displayName", "ext", "serverid", "serverindex", "servername", "rolename", "roleid", "accountid", "roledid", "maybeFirst", "biOpt"}, ResponseSchema: "StateDelta"},
	{Name: RPCSdkSendGoods, Group: "sdk", Method: "sendGoods", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSecPwdChangePwd, Group: "secPwd", Method: "changePwd", RequestShape: RPCRequestFields, RequestFields: []string{"oldPwd", "newPwd"}, ResponseSchema: "StateDelta"},
	{Name: RPCSecPwdCheckPwd, Group: "secPwd", Method: "checkPwd", RequestShape: RPCRequestFields, RequestFields: []string{"pwd"}, ResponseSchema: "StateDelta"},
	{Name: RPCSecPwdCloseSecPwd, Group: "secPwd", Method: "closeSecPwd", RequestShape: RPCRequestFields, RequestFields: []string{"pwd"}, ResponseSchema: "StateDelta"},
	{Name: RPCSecPwdFirstUse, Group: "secPwd", Method: "firstUse", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSecPwdGetQuestion, Group: "secPwd", Method: "getQuestion", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSecPwdResetPwd, Group: "secPwd", Method: "resetPwd", RequestShape: RPCRequestFields, RequestFields: []string{"newPwd", "selectIdx", "answer"}, ResponseSchema: "StateDelta"},
	{Name: RPCSecPwdSetPwd, Group: "secPwd", Method: "setPwd", RequestShape: RPCRequestFields, RequestFields: []string{"pwd", "question", "answer", "question2", "answer2"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopBuy, Group: "shop", Method: "buy", RequestShape: RPCRequestFields, RequestFields: []string{"tempId", "itemId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopEnter, Group: "shop", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"tempId"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopRefresh, Group: "shop", Method: "refresh", RequestShape: RPCRequestFields, RequestFields: []string{"tempId", "type"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopSync, Group: "shop", Method: "sync", RequestShape: RPCRequestFields, RequestFields: []string{"tempIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopCultivateBuy, Group: "shopCultivate", Method: "buy", RequestShape: RPCRequestFields, RequestFields: []string{"shopId"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopCultivateBuyOneKey, Group: "shopCultivate", Method: "buyOneKey", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCShopCultivateEnter, Group: "shopCultivate", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCShopCultivateRefresh, Group: "shopCultivate", Method: "refresh", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCShopFlowerElvesBuy, Group: "shopFlowerElves", Method: "buy", RequestShape: RPCRequestFields, RequestFields: []string{"shopId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopFlowerElvesEnter, Group: "shopFlowerElves", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCShopFmlRaceBuy, Group: "shopFmlRace", Method: "buy", RequestShape: RPCRequestFields, RequestFields: []string{"isAll"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopFmlUsrBuildShop, Group: "shopFmlUsr", Method: "buildShop", RequestShape: RPCRequestFields, RequestFields: []string{"skillId"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopFmlUsrBuy, Group: "shopFmlUsr", Method: "buy", RequestShape: RPCRequestFields, RequestFields: []string{"slotId", "count"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopFmlUsrBuyAll, Group: "shopFmlUsr", Method: "buyAll", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCShopFmlUsrEnter, Group: "shopFmlUsr", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCShopFmlUsrRefresh, Group: "shopFmlUsr", Method: "refresh", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCShopFmlUsrUnlockSlot, Group: "shopFmlUsr", Method: "unlockSlot", RequestShape: RPCRequestFields, RequestFields: []string{"slotId"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopGiftbagBuy, Group: "shopGiftbag", Method: "buy", RequestShape: RPCRequestFields, RequestFields: []string{"shopId", "num"}, ResponseSchema: "StateDelta"},
	{Name: RPCShopGiftbagEnter, Group: "shopGiftbag", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSignRecvGradeRwd, Group: "sign", Method: "recvGradeRwd", RequestShape: RPCRequestFields, RequestFields: []string{"gradeIdx"}, ResponseSchema: "StateDelta"},
	{Name: RPCSignSign, Group: "sign", Method: "sign", RequestShape: RPCRequestFields, RequestFields: []string{"patchDay"}, ResponseSchema: "StateDelta"},
	{Name: RPCSignSignSeven, Group: "sign", Method: "sign_seven", RequestShape: RPCRequestFields, RequestFields: []string{"day"}, ResponseSchema: "StateDelta"},
	{Name: RPCSignTypeEnter, Group: "signType", Method: "enter", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSignTypeRecv, Group: "signType", Method: "recv", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSignTypeSign, Group: "signType", Method: "sign", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCStoryMainEnter, Group: "storyMain", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCStoryMainUnlock, Group: "storyMain", Method: "unlock", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSysInformChat, Group: "sys", Method: "informChat", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSysInformFml, Group: "sys", Method: "informFml", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCSysInformUsr, Group: "sys", Method: "informUsr", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCTaskAchRecv, Group: "taskAch", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"id"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskAchRecvOneKey, Group: "taskAch", Method: "recvOneKey", RequestShape: RPCRequestFields, RequestFields: []string{"id"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskDlyEnter, Group: "taskDly", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCTaskDlyRecv, Group: "taskDly", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"id"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskDlyRecvBox, Group: "taskDly", Method: "recvBox", RequestShape: RPCRequestFields, RequestFields: []string{"idx"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskInvRecv, Group: "taskInv", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"id", "isPro"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskInvRecvOneKey, Group: "taskInv", Method: "recvOneKey", RequestShape: RPCRequestFields, RequestFields: []string{"id"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskMainRecv, Group: "taskMain", Method: "recv", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCTaskSysGiftBuy, Group: "taskSys", Method: "giftBuy", RequestShape: RPCRequestFields, RequestFields: []string{"giftId"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskSysRecv, Group: "taskSys", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskSysRecvLvlRwd, Group: "taskSys", Method: "recvLvlRwd", RequestShape: RPCRequestFields, RequestFields: []string{"classify", "lvl"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskSysRecvOneKey, Group: "taskSys", Method: "recvOneKey", RequestShape: RPCRequestFields, RequestFields: []string{"classify", "grpId"}, ResponseSchema: "StateDelta"},
	{Name: RPCTaskWeekRecv, Group: "taskWeek", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"id"}, ResponseSchema: "StateDelta"},
	{Name: RPCTeamOrderPopupShowT, Group: "teamOrderPopup", Method: "showT", RequestShape: RPCRequestFields, RequestFields: []string{"point"}, ResponseSchema: "StateDelta"},
	{Name: RPCThirdpartyApplyToken, Group: "thirdparty", Method: "applyToken", RequestShape: RPCRequestFields, RequestFields: []string{"type", "uid"}, ResponseSchema: "StateDelta"},
	{Name: RPCTitleActiveTitle, Group: "title", Method: "activeTitle", RequestShape: RPCRequestFields, RequestFields: []string{"titleId"}, ResponseSchema: "StateDelta"},
	{Name: RPCTitleChgTitle, Group: "title", Method: "chgTitle", RequestShape: RPCRequestFields, RequestFields: []string{"titleId"}, ResponseSchema: "StateDelta"},
	{Name: RPCTitleSetTitleShow, Group: "title", Method: "setTitleShow", RequestShape: RPCRequestFields, RequestFields: []string{"titleIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCTokenInfoGetToken, Group: "tokenInfo", Method: "getToken", RequestShape: RPCRequestFields, RequestFields: []string{"type", "param"}, ResponseSchema: "StateDelta"},
	{Name: RPCTtMoneyTaskGenGldOrder, Group: "ttMoneyTask", Method: "genGldOrder", RequestShape: RPCRequestFields, RequestFields: []string{"taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCTtMoneyTaskRecv, Group: "ttMoneyTask", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCTtMoneyTaskRefresh, Group: "ttMoneyTask", Method: "refresh", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrActiveCard, Group: "usr", Method: "activeCard", RequestShape: RPCRequestFields, RequestFields: []string{"cardId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrActiveEmoji, Group: "usr", Method: "activeEmoji", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrActiveHead, Group: "usr", Method: "activeHead", RequestShape: RPCRequestFields, RequestFields: []string{"headId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrActiveMedal, Group: "usr", Method: "activeMedal", RequestShape: RPCRequestFields, RequestFields: []string{"medalId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrAfterShare, Group: "usr", Method: "afterShare", RequestShape: RPCRequestFields, RequestFields: []string{"shareId", "ext"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrChgCard, Group: "usr", Method: "chgCard", RequestShape: RPCRequestFields, RequestFields: []string{"cardId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrChgFace, Group: "usr", Method: "chgFace", RequestShape: RPCRequestFields, RequestFields: []string{"faceId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrChgIco, Group: "usr", Method: "chgIco", RequestShape: RPCRequestFields, RequestFields: []string{"ico"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrChgName, Group: "usr", Method: "chgName", RequestShape: RPCRequestFields, RequestFields: []string{"name", "isFree"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrChgSex, Group: "usr", Method: "chgSex", RequestShape: RPCRequestFields, RequestFields: []string{"sexId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrChgSign, Group: "usr", Method: "chgSign", RequestShape: RPCRequestFields, RequestFields: []string{"sign"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrClearVipService, Group: "usr", Method: "clearVipService", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrGetSalary, Group: "usr", Method: "getSalary", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrHeartTick, Group: "usr", Method: "heartTick", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLazySync, Group: "usr", Method: "lazySync", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrRecvSignRwd, Group: "usr", Method: "recvSignRwd", RequestShape: RPCRequestFields, RequestFields: []string{"medalId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrRefreshMedal, Group: "usr", Method: "refreshMedal", RequestShape: RPCRequestFields, RequestFields: []string{"medalId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrSaveAuthInfo, Group: "usr", Method: "saveAuthInfo", RequestShape: RPCRequestFields, RequestFields: []string{"authInfo"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrSaveCustIco, Group: "usr", Method: "saveCustIco", RequestShape: RPCRequestFields, RequestFields: []string{"icoMD5", "ico64"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrSetMedalShow, Group: "usr", Method: "setMedalShow", RequestShape: RPCRequestFields, RequestFields: []string{"medalIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrShare, Group: "usr", Method: "share", RequestShape: RPCRequestFields, RequestFields: []string{"shareId", "ext"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrTriggerEvent, Group: "usr", Method: "triggerEvent", RequestShape: RPCRequestFields, RequestFields: []string{"key", "subKey", "param"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrUpdateGuide, Group: "usr", Method: "updateGuide", RequestShape: RPCRequestFields, RequestFields: []string{"guideId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrUpdateSoftGuide, Group: "usr", Method: "updateSoftGuide", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrUpdateUsrExt, Group: "usr", Method: "updateUsrExt", RequestShape: RPCRequestFields, RequestFields: []string{"k", "v"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrUpdateUsrSet, Group: "usr", Method: "updateUsrSet", RequestShape: RPCRequestFields, RequestFields: []string{"type", "value"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrUpdateVipService, Group: "usr", Method: "updateVipService", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrUpgrade, Group: "usr", Method: "upgrade", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrWorship, Group: "usr", Method: "worship", RequestShape: RPCRequestFields, RequestFields: []string{"type"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraRecvAntiFraudQARwd, Group: "usrExtra", Method: "recvAntiFraudQARwd", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraRecvTtFansRwd, Group: "usrExtra", Method: "recvTtFansRwd", RequestShape: RPCRequestFields, RequestFields: []string{"termId", "rewardId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraRecvVersionUpdateRwd, Group: "usrExtra", Method: "recvVersionUpdateRwd", RequestShape: RPCRequestFields, RequestFields: []string{"version"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraRecvWbFansRwd, Group: "usrExtra", Method: "recvWbFansRwd", RequestShape: RPCRequestFields, RequestFields: []string{"termId", "rewardId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraReportUsr, Group: "usrExtra", Method: "reportUsr", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraSetMsPwd, Group: "usrExtra", Method: "setMsPwd", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraSetShowAddress, Group: "usrExtra", Method: "setShowAddress", RequestShape: RPCRequestFields, RequestFields: []string{"showAddress"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraShareMsg, Group: "usrExtra", Method: "shareMsg", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraSyncAddress, Group: "usrExtra", Method: "syncAddress", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraUpdateAntiFraudQAStatus, Group: "usrExtra", Method: "updateAntiFraudQAStatus", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraUpdateTaskMap, Group: "usrExtra", Method: "updateTaskMap", RequestShape: RPCRequestFields, RequestFields: []string{"taskId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrExtraUpdateTtSubscribe, Group: "usrExtra", Method: "updateTtSubscribe", RequestShape: RPCRequestFields, RequestFields: []string{"status"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandClear, Group: "usrLand", Method: "clear", RequestShape: RPCRequestFields, RequestFields: []string{"landId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandClearBatch, Group: "usrLand", Method: "clearBatch", RequestShape: RPCRequestFields, RequestFields: []string{"landIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandClearOneKey, Group: "usrLand", Method: "clearOneKey", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandHarvest, Group: "usrLand", Method: "harvest", RequestShape: RPCRequestFields, RequestFields: []string{"landId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandHarvestOneKey, Group: "usrLand", Method: "harvestOneKey", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandPlant, Group: "usrLand", Method: "plant", RequestShape: RPCRequestFields, RequestFields: []string{"landId", "flowerId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandPlantBatch, Group: "usrLand", Method: "plantBatch", RequestShape: RPCRequestFields, RequestFields: []string{"landIds", "flowerId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandPlantOneKey, Group: "usrLand", Method: "plantOneKey", RequestShape: RPCRequestFields, RequestFields: []string{"flowerId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandRefresh, Group: "usrLand", Method: "refresh", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandSpeedUp, Group: "usrLand", Method: "speedUp", RequestShape: RPCRequestFields, RequestFields: []string{"landId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandSpeedUpBatch, Group: "usrLand", Method: "speedUpBatch", RequestShape: RPCRequestFields, RequestFields: []string{"landIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandSpeedUpFree, Group: "usrLand", Method: "speedUpFree", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandSpeedUpOneKey, Group: "usrLand", Method: "speedUpOneKey", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandUnlockLand, Group: "usrLand", Method: "unlockLand", RequestShape: RPCRequestFields, RequestFields: []string{"landId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandWater, Group: "usrLand", Method: "water", RequestShape: RPCRequestFields, RequestFields: []string{"landId"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandWaterBatch, Group: "usrLand", Method: "waterBatch", RequestShape: RPCRequestFields, RequestFields: []string{"landIds"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrLandWaterOneKey, Group: "usrLand", Method: "waterOneKey", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrRedDelRed, Group: "usrRed", Method: "delRed", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCUsrSubscribePushAddSubscribeNum, Group: "usrSubscribePush", Method: "addSubscribeNum", RequestShape: RPCRequestFields, RequestFields: []string{"typeList"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrSubscribePushAddSubscribeNumPermanent, Group: "usrSubscribePush", Method: "addSubscribeNumPermanent", RequestShape: RPCRequestFields, RequestFields: []string{"typeList"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrSubscribePushMsgPushSetting, Group: "usrSubscribePush", Method: "msgPushSetting", RequestShape: RPCRequestFields, RequestFields: []string{"settingMap"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrSubscribePushMsgPushSettingGlobal, Group: "usrSubscribePush", Method: "msgPushSettingGlobal", RequestShape: RPCRequestFields, RequestFields: []string{"isOpen", "isSubscribeOpen"}, ResponseSchema: "StateDelta"},
	{Name: RPCUsrVerInfoRefresh, Group: "usrVerInfo", Method: "refresh", RequestShape: RPCRequestFields, RequestFields: []string{"point"}, ResponseSchema: "StateDelta"},
	{Name: RPCValentinesApply, Group: "valentines", Method: "apply", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCValentinesBind, Group: "valentines", Method: "bind", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCValentinesEnter, Group: "valentines", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCValentinesFrdStates, Group: "valentines", Method: "frdStates", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "uids"}, ResponseSchema: "StateDelta"},
	{Name: RPCValentinesRecv, Group: "valentines", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCValentinesReject, Group: "valentines", Method: "reject", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCValentinesUnBind, Group: "valentines", Method: "unBind", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCVerifyCheckVerification, Group: "verify", Method: "checkVerification", RequestShape: RPCRequestFields, RequestFields: []string{"type", "requestId", "code"}, ResponseSchema: "StateDelta"},
	{Name: RPCVerifyRefreshVerification, Group: "verify", Method: "refreshVerification", RequestShape: RPCRequestFields, RequestFields: []string{"type", "isManual"}, ResponseSchema: "StateDelta"},
	{Name: RPCVipRecv, Group: "vip", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"vip"}, ResponseSchema: "StateDelta"},
	{Name: RPCWaterRqstDjst, Group: "waterRqst", Method: "djst", RequestShape: RPCRequestFields, RequestFields: []string{"point"}, ResponseSchema: "StateDelta"},
	{Name: RPCWaterwheelEnter, Group: "waterwheel", Method: "enter", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCWaterwheelRecv, Group: "waterwheel", Method: "recv", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCWaterwheelSkip, Group: "waterwheel", Method: "skip", RequestShape: RPCRequestEmpty, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteDay26Apply, Group: "whiteDay26", Method: "apply", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteDay26Bind, Group: "whiteDay26", Method: "bind", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteDay26Enter, Group: "whiteDay26", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteDay26FrdStates, Group: "whiteDay26", Method: "frdStates", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "uids"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteDay26Recv, Group: "whiteDay26", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteDay26Reject, Group: "whiteDay26", Method: "reject", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteDay26UnBind, Group: "whiteDay26", Method: "unBind", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteValentineApply, Group: "whiteValentine", Method: "apply", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteValentineBind, Group: "whiteValentine", Method: "bind", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteValentineEnter, Group: "whiteValentine", Method: "enter", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteValentineFrdStates, Group: "whiteValentine", Method: "frdStates", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "uids"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteValentineRecv, Group: "whiteValentine", Method: "recv", RequestShape: RPCRequestFields, RequestFields: []string{"batchId"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteValentineReject, Group: "whiteValentine", Method: "reject", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCWhiteValentineUnBind, Group: "whiteValentine", Method: "unBind", RequestShape: RPCRequestFields, RequestFields: []string{"batchId", "dUid"}, ResponseSchema: "StateDelta"},
	{Name: RPCZooAddFoodstuff, Group: "zoo", Method: "addFoodstuff", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooCalNaturalAtt, Group: "zoo", Method: "calNaturalAtt", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooChangePetName, Group: "zoo", Method: "changePetName", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooEnterZoo, Group: "zoo", Method: "enterZoo", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooFeedOtherPet, Group: "zoo", Method: "feedOtherPet", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooFeedPets, Group: "zoo", Method: "feedPets", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooFindPet, Group: "zoo", Method: "findPet", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooFindPetByUsrBack, Group: "zoo", Method: "findPetByUsrBack", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooGetGuideEventRwd, Group: "zoo", Method: "getGuideEventRwd", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooHandBeOverdueEvent, Group: "zoo", Method: "handBeOverdueEvent", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooHandleEvent, Group: "zoo", Method: "handleEvent", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooInitZoo, Group: "zoo", Method: "initZoo", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooReadLog, Group: "zoo", Method: "readLog", RequestShape: RPCRequestFields, RequestFields: []string{"petId"}, ResponseSchema: "StateDelta"},
	{Name: RPCZooReadSouvenir, Group: "zoo", Method: "readSouvenir", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooRecvSouvenirRwd, Group: "zoo", Method: "recvSouvenirRwd", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooRefreshPetStatus, Group: "zoo", Method: "refreshPetStatus", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooSetUpSleepTime, Group: "zoo", Method: "setUpSleepTime", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooStrokePet, Group: "zoo", Method: "strokePet", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooUsePetItem, Group: "zoo", Method: "usePetItem", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooVisitZoo, Group: "zoo", Method: "visitZoo", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooDecorateEquip, Group: "zooDecorate", Method: "equip", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
	{Name: RPCZooDecorateRead, Group: "zooDecorate", Method: "read", RequestShape: RPCRequestRaw, RequestFields: nil, ResponseSchema: "StateDelta"},
}

var gameJSRPCSpecMap = func() map[RPCName]RPCSpec {
	out := make(map[RPCName]RPCSpec, len(gameJSRPCSpecs))
	for _, spec := range gameJSRPCSpecs {
		out[spec.Name] = spec
	}
	return out
}()

// KnownRPCNames returns the RPC names statically observed in the unpacked
// client. The returned slice is safe for callers to mutate.
func KnownRPCNames() []RPCName {
	out := append([]RPCName(nil), gameJSRPCNames...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// KnownRPCSpecs returns metadata for every statically observed game.js RPC.
func KnownRPCSpecs() []RPCSpec {
	out := append([]RPCSpec(nil), gameJSRPCSpecs...)
	for i := range out {
		out[i].RequestFields = append([]string(nil), out[i].RequestFields...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LookupRPCSpec returns static metadata for one observed game.js RPC.
func LookupRPCSpec(name string) (RPCSpec, bool) {
	normalized, err := NormalizeRPCName(name)
	if err != nil {
		return RPCSpec{}, false
	}
	spec, ok := gameJSRPCSpecMap[normalized]
	if !ok {
		return RPCSpec{}, false
	}
	spec.RequestFields = append([]string(nil), spec.RequestFields...)
	return spec, true
}

// IsKnownRPCName reports whether name was observed in game.js. Unknown names
// may still work if the server supports them; this is only a discovery aid.
func IsKnownRPCName(name string) bool {
	normalized, err := NormalizeRPCName(name)
	if err != nil {
		return false
	}
	_, ok := gameJSRPCNameSet[normalized]
	return ok
}
